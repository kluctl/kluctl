package deployment

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"k8s.io/apimachinery/pkg/api/errors"
	"sort"
	"strings"
	"sync"
)

type Images struct {
	fixedImages []types.FixedImage
	seenImages  []types.FixedImage
	mutex       sync.Mutex
}

func NewImages() (*Images, error) {
	return &Images{}, nil
}

func (images *Images) AddFixedImage(fi types.FixedImage) {
	images.fixedImages = append(images.fixedImages, fi)
}

func (images *Images) PrependFixedImages(fis []types.FixedImage) {
	var newFixedImages []types.FixedImage
	newFixedImages = append(newFixedImages, fis...)
	newFixedImages = append(newFixedImages, images.fixedImages...)
	images.fixedImages = newFixedImages
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
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Image < ret[j].Image
	})
	return ret
}

func (images *Images) parseFixedImagesFromVars(vars *uo.UnstructuredObject) ([]types.FixedImage, error) {
	fisU, _, err := vars.GetNestedObjectList("images")
	if err != nil {
		return nil, fmt.Errorf("failed to parse fixed images from vars: %w", err)
	}
	fis := make([]types.FixedImage, 0, len(fisU))
	for i, u := range fisU {
		var fi types.FixedImage
		err = u.ToStruct(&fi)
		if err != nil {
			return nil, fmt.Errorf("failed to parse fixed image from vars at index %d: %w", i, err)
		}
		fis = append(fis, fi)
	}
	return fis, nil
}

func (images *Images) getFixedImage(image string, namespace string, deployment string, container string, vars *uo.UnstructuredObject) (*string, error) {
	cmpList := func(fis []types.FixedImage) *string {
		for i := len(fis) - 1; i >= 0; i-- {
			fi := fis[i]
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

	fisFromVars, err := images.parseFixedImagesFromVars(vars)
	if err != nil {
		return nil, err
	}

	fi := cmpList(images.fixedImages)
	if fi != nil {
		return fi, nil
	}
	fi = cmpList(fisFromVars)
	if fi != nil {
		return fi, nil
	}

	return nil, nil
}

const beginPlaceholder = "XXXXXbegin_get_image_"
const endPlaceholder = "_end_get_imageXXXXX"

type placeHolder struct {
	Image            string `json:"image"`
	HasLatestVersion bool   `json:"hasLatestVersion"`

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

func (images *Images) ResolvePlaceholders(ctx context.Context, k *k8s.K8sCluster, o *uo.UnstructuredObject, deploymentDir string, tags []string, vars *uo.UnstructuredObject) error {
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

		resultImage, err := images.resolveImage(ctx, ph, ref, deployment, deployed, deploymentDir, tags, vars)
		if err != nil {
			return err
		}
		if resultImage == nil {
			return fmt.Errorf("failed to find fixed image for %s", ph.Image)
		}

		ph.FieldValue = ph.FieldValue[:ph.StartOffset] + *resultImage + ph.FieldValue[ph.EndOffset:]
		err = o.SetNestedField(ph.FieldValue, ph.FieldPath...)
		if err != nil {
			return err
		}
	}
	return nil
}

func (images *Images) resolveImage(ctx context.Context, ph placeHolder, ref k8s2.ObjectRef, deployment string, deployed *string, deploymentDir string, tags []string, vars *uo.UnstructuredObject) (*string, error) {
	if ph.HasLatestVersion {
		status.Deprecation(ctx, "latest-version-filter", "latest_version is deprecated when using images.get_image() and is completely ignored. Please remove usages of latest_version as it will fail to render in a future kluctl release.")
	}

	result, err := images.getFixedImage(ph.Image, ref.Namespace, deployment, ph.Container, vars)
	if err != nil {
		return nil, err
	}

	si := types.FixedImage{
		Image:         ph.Image,
		DeployedImage: deployed,
		Namespace:     &ref.Namespace,
		Object:        &ref,
		Deployment:    &deployment,
		Container:     &ph.Container,
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
