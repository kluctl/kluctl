package deployment

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/codablock/kluctl/pkg/yaml"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"strings"
	"sync"
)

type Images struct {
	updateImages bool
	fixedImages  []types.FixedImage
	seenImages   []types.FixedImage
	mutex        sync.Mutex
}

func NewImages(updateImages bool) (*Images, error) {
	return &Images{
		updateImages: updateImages,
	}, nil
}

func (images *Images) AddFixedImage(fi types.FixedImage) {
	images.fixedImages = append(images.fixedImages, fi)
}

func (images *Images) GetFixedImage(image string, namespace string, deployment string, container string) *string {
	for i := len(images.fixedImages) - 1; i >= 0; i-- {
		fi := &images.fixedImages[i]
		if fi.Image != image {
			continue
		}
		if fi.Namespace != nil && namespace != *fi.Namespace {
			continue
		}
		if fi.Deployment != nil && deployment != *fi.Deployment {
			continue
		}
		if fi.Container != nil && container != *fi.Container {
			continue
		}
		return &fi.ResultImage
	}
	return nil
}

func (images *Images) GetLatestImageFromRegistry(image string, latestVersion string) (*string, error) {
	return nil, nil
}

const beginPlaceholder = "XXXXXbegin_get_image_"
const endPlaceholder = "_end_get_imageXXXXX"

type placeHolder struct {
	Image         string `yaml:"image"`
	LatestVersion string `yaml:"latestVersion"`

	startOffset int
	endOffset   int
}

func (images *Images) parsePlaceholder(s string, offset int) (*placeHolder, error) {
	start := strings.Index(s[offset:], beginPlaceholder)
	if start == -1 {
		return nil, nil
	}
	end := strings.Index(s[start:], endPlaceholder)
	if end == -1 {
		return nil, fmt.Errorf("beginPlaceholder marker without endPlaceholder marker")
	}
	b64 := s[start+len(beginPlaceholder) : end]
	b, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return nil, err
	}
	var ph placeHolder
	err = yaml.ReadYamlStream(bytes.NewReader(b), &ph)
	if err != nil {
		return nil, err
	}
	ph.startOffset = start
	ph.endOffset = end + len(endPlaceholder)

	return &ph, nil
}

func (images *Images) extractContainerName(parent interface{}) string {
	parentM, ok := parent.(map[string]interface{})
	if ok {
		if x, ok := parentM["name"]; ok {
			if y, ok := x.(string); ok {
				return y
			}
		}
	}
	return ""
}

func (images *Images) ResolvePlaceholders(k *k8s.K8sCluster, o *unstructured.Unstructured, deploymentDir string, tags []string) error {
	namespace := o.GetNamespace()
	deployment := fmt.Sprintf("%s/%s", o.GetKind(), o.GetName())

	var remoteObject *unstructured.Unstructured
	triedRemoteObject := false

	err := uo.NewObjectIterator(o.Object).IterateLeafs(func(it *uo.ObjectIterator) error {
		s, ok := it.Value().(string)
		if !ok {
			return nil
		}
		newS := ""
		container := images.extractContainerName(it.Parent())

		offset := 0
		for true {
			ph, err := images.parsePlaceholder(s, offset)
			if err != nil {
				return err
			}
			if ph == nil {
				newS += s[offset:]
				break
			} else {
				newS += s[offset:ph.startOffset]
			}

			if !triedRemoteObject {
				triedRemoteObject = true
				remoteObject, _, err = k.GetSingleObject(types.RefFromObject(o))
				if err != nil && !errors.IsNotFound(err) {
					return err
				}
			}

			var deployed *string
			if remoteObject != nil && ph.startOffset == 0 && ph.endOffset == len(s) {
				x, found, _ := uo.FromUnstructured(remoteObject).GetNestedField(it.KeyPath()...)
				if found {
					if y, ok := x.(string); ok {
						deployed = &y
					}
				}
			}

			resultImage, err := images.resolveImage(ph, namespace, deployment, container, deployed, deploymentDir, tags)
			if err != nil {
				return err
			}
			if resultImage == nil {
				return fmt.Errorf("failed to find image for %s and latest version %s", ph.Image, ph.LatestVersion)
			}
			newS += *resultImage

			offset = ph.endOffset
			if offset >= len(s) {
				break
			}
		}
		return it.SetValue(newS)
	})
	if err != nil {
		return err
	}
	return nil
}

func (images *Images) resolveImage(ph *placeHolder, namespace string, deployment string, container string, deployed *string, deploymentDir string, tags []string) (*string, error) {
	fixed := images.GetFixedImage(ph.Image, namespace, deployment, container)

	registry, err := images.GetLatestImageFromRegistry(ph.Image, ph.LatestVersion)
	if err != nil {
		return nil, err
	}

	result := deployed
	if result == nil || images.updateImages {
		result = registry
	}
	if !images.updateImages && fixed != nil {
		result = fixed
	}

	if result != nil {
		si := types.FixedImage{
			Image:         ph.Image,
			ResultImage:   *result,
			DeployedImage: deployed,
			RegistryImage: registry,
			Namespace:     &namespace,
			Deployment:    &deployment,
			Container:     &container,
			VersionFilter: &ph.LatestVersion,
			DeployTags:    tags,
			DeploymentDir: &deploymentDir,
		}
		images.mutex.Lock()
		images.seenImages = append(images.seenImages, si)
		images.mutex.Unlock()
	}
	return result, nil
}
