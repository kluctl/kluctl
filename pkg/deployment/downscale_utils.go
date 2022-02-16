package deployment

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/codablock/kluctl/pkg/yaml"
	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"regexp"
	"strconv"
)

var (
	downscaleAnnotationPatchRegex = regexp.MustCompile(`^kluctl.io/downscale-patch(-\d*)?$`)
	downscaleAnnotationDelete     = "kluctl.io/downscale-delete"
	downscaleAnnotationIgnore     = "kluctl.io/downscale-ignore"
)

func isDownscaleDelete(o *unstructured.Unstructured) bool {
	a, _ := o.GetAnnotations()[downscaleAnnotationDelete]
	b, _ := strconv.ParseBool(a)
	return b
}

func isDownscaleIgnore(o *unstructured.Unstructured) bool {
	a, _ := o.GetAnnotations()[downscaleAnnotationIgnore]
	b, _ := strconv.ParseBool(a)
	return b
}

func downscaleObject(remote *unstructured.Unstructured, local *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	ret := remote
	if isDownscaleIgnore(local) {
		return ret, nil
	}
	var patch jsonpatch.Patch
	for k, v := range local.GetAnnotations() {
		if downscaleAnnotationPatchRegex.MatchString(k) {
			j, err := yaml.ConvertYamlToJson([]byte(v))
			if err != nil {
				return nil, fmt.Errorf("invalid jsonpatch json/yaml: %w", err)
			}
			p, err := jsonpatch.DecodePatch(j)
			if err != nil {
				return nil, fmt.Errorf("invalid jsonpatch: %w", err)
			}
			patch = append(patch, p...)
		}
	}

	if len(patch) != 0 {
		j, err := yaml.WriteYamlBytes(ret.Object)
		if err != nil {
			return nil, err
		}
		j, err = yaml.ConvertYamlToJson(j)
		if err != nil {
			return nil, err
		}
		j, err = patch.Apply(j)
		if err != nil {
			return nil, err
		}
		ret = &unstructured.Unstructured{}
		err = yaml.ReadYamlBytes(j, &ret.Object)
		if err != nil {
			return nil, err
		}
	}

	ref := types.RefFromObject(remote)
	switch ref.GVK.GroupKind() {
	case schema.GroupKind{Group: "apps", Kind: "Deployment"}:
		fallthrough
	case schema.GroupKind{Group: "apps", Kind: "StatefulSet"}:
		ret = uo.CopyUnstructured(ret)
		err := uo.FromUnstructured(ret).SetNestedField(0, "spec", "replicas")
		if err != nil {
			return nil, err
		}
	case schema.GroupKind{Group: "batch", Kind: "CronJob"}:
		ret = uo.CopyUnstructured(ret)
		err := uo.FromUnstructured(ret).SetNestedField(true, "spec", "suspend")
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}
