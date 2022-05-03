package deployment

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/registries"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/utils/versions"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"k8s.io/apimachinery/pkg/api/errors"
	"strings"
	"sync"
)

type Images struct {
	rh           *registries.RegistryHelper
	updateImages bool
	offline      bool
	fixedImages  []types.FixedImage
	seenImages   []types.FixedImage
	mutex        sync.Mutex

	registryCache utils.ThreadSafeMultiCache
}

func NewImages(rh *registries.RegistryHelper, updateImages bool, offline bool) (*Images, error) {
	return &Images{
		rh:           rh,
		updateImages: updateImages,
		offline:      offline,
	}, nil
}

func (images *Images) AddFixedImage(fi types.FixedImage) {
	images.fixedImages = append(images.fixedImages, fi)
}

func (images *Images) SeenImages(simple bool) []types.FixedImage {
	var ret []types.FixedImage
	for _, fi := range images.seenImages {
		if simple {
			ret = append(ret, types.FixedImage{
				Image:       fi.Image,
				ResultImage: fi.ResultImage,
			})
		} else {
			ret = append(ret, fi)
		}
	}
	return ret
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
	if images.offline {
		return nil, nil
	}

	ret, err := images.registryCache.Get(image, "tag", func() (interface{}, error) {
		return images.rh.ListImageTags(image)
	})
	if err != nil {
		return nil, err
	}
	tags, _ := ret.([]string)

	if len(tags) == 0 {
		return nil, nil
	}

	lv, err := versions.ParseLatestVersion(latestVersion)
	if err != nil {
		return nil, err
	}

	tags = versions.Filter(lv, tags)
	if len(tags) == 0 {
		return nil, fmt.Errorf("no tag matched latest_version: %s", latestVersion)
	}

	latest := lv.Latest(tags)
	result := fmt.Sprintf("%s:%s", image, latest)
	return &result, nil
}

const beginPlaceholder = "XXXXXbegin_get_image_"
const endPlaceholder = "_end_get_imageXXXXX"

type placeHolder struct {
	Image         string `yaml:"image"`
	LatestVersion string `yaml:"latestVersion"`

	Container string

	FieldPath   []interface{}
	FieldValue  string
	StartOffset int
	EndOffset   int
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
	ph.FieldValue = s
	ph.StartOffset = start
	ph.EndOffset = end + len(endPlaceholder)

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

func (images *Images) FindPlaceholders(o *uo.UnstructuredObject) ([]placeHolder, error) {
	var ret []placeHolder

	err := uo.NewObjectIterator(o.Object).IterateLeafs(func(it *uo.ObjectIterator) error {
		s, ok := it.Value().(string)
		if !ok {
			return nil
		}

		container := images.extractContainerName(it.Parent())

		offset := 0
		for true {
			ph, err := images.parsePlaceholder(s, offset)
			if err != nil {
				return err
			}
			if ph == nil {
				break
			}
			ph.FieldPath = it.KeyPathCopy()
			ph.Container = container
			ret = append(ret, *ph)
			offset = ph.EndOffset
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (images *Images) ResolvePlaceholders(k *k8s.K8sCluster, o *uo.UnstructuredObject, deploymentDir string, tags []string) error {
	placeholders, err := images.FindPlaceholders(o)
	if err != nil {
		return err
	}

	ref := o.GetK8sRef()
	deployment := fmt.Sprintf("%s/%s", ref.GVK.Kind, ref.Name)

	var remoteObject *uo.UnstructuredObject
	triedRemoteObject := false

	// iterate backwards so that replacements are easy
	for i := len(placeholders) - 1; i >= 0; i-- {
		ph := placeholders[i]

		if !triedRemoteObject {
			triedRemoteObject = true
			if k != nil {
				remoteObject, _, err = k.GetSingleObject(o.GetK8sRef())
				if err != nil && !errors.IsNotFound(err) {
					return err
				}
			}
		}

		var deployed *string
		if remoteObject != nil && ph.StartOffset == 0 && ph.EndOffset == len(ph.FieldValue) {
			x, found, _ := remoteObject.GetNestedField(ph.FieldPath...)
			if found {
				if y, ok := x.(string); ok {
					deployed = &y
				}
			}
		}

		resultImage, err := images.resolveImage(ph, ref, deployment, deployed, deploymentDir, tags)
		if err != nil {
			return err
		}
		if resultImage == nil {
			return fmt.Errorf("failed to find image for %s and latest version %s", ph.Image, ph.LatestVersion)
		}

		ph.FieldValue = ph.FieldValue[:ph.StartOffset] + *resultImage + ph.FieldValue[ph.EndOffset:]
		err = o.SetNestedField(ph.FieldValue, ph.FieldPath...)
		if err != nil {
			return err
		}
	}
	return nil
}

func (images *Images) resolveImage(ph placeHolder, ref k8s2.ObjectRef, deployment string, deployed *string, deploymentDir string, tags []string) (*string, error) {
	fixed := images.GetFixedImage(ph.Image, ref.Namespace, deployment, ph.Container)

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

	si := types.FixedImage{
		Image:         ph.Image,
		DeployedImage: deployed,
		RegistryImage: registry,
		Namespace:     &ref.Namespace,
		Object:        &ref,
		Deployment:    &deployment,
		Container:     &ph.Container,
		VersionFilter: &ph.LatestVersion,
		DeployTags:    tags,
		DeploymentDir: &deploymentDir,
	}
	if result != nil {
		si.ResultImage = *result
	}
	images.mutex.Lock()
	images.seenImages = append(images.seenImages, si)
	images.mutex.Unlock()
	return result, nil
}
