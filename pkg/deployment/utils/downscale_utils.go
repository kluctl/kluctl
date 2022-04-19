package utils

import (
	"fmt"
	jsonpatch "github.com/evanphx/json-patch"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"regexp"
	"strconv"
)

var (
	downscaleAnnotationPatchRegex = regexp.MustCompile(`^kluctl.io/downscale-patch(-\d*)?$`)
	downscaleAnnotationDelete     = "kluctl.io/downscale-delete"
	downscaleAnnotationIgnore     = "kluctl.io/downscale-ignore"
)

func IsDownscaleDelete(o *uo.UnstructuredObject) bool {
	a, _ := o.GetK8sAnnotations()[downscaleAnnotationDelete]
	b, _ := strconv.ParseBool(a)
	return b
}

func isDownscaleIgnore(o *uo.UnstructuredObject) bool {
	a, _ := o.GetK8sAnnotations()[downscaleAnnotationIgnore]
	b, _ := strconv.ParseBool(a)
	return b
}

func DownscaleObject(remote *uo.UnstructuredObject, local *uo.UnstructuredObject) (*uo.UnstructuredObject, error) {
	ret := remote
	if isDownscaleIgnore(local) {
		return ret, nil
	}
	var patch jsonpatch.Patch
	for _, v := range local.GetK8sAnnotationsWithRegex(downscaleAnnotationPatchRegex) {
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
		ret = &uo.UnstructuredObject{}
		err = yaml.ReadYamlBytes(j, &ret.Object)
		if err != nil {
			return nil, err
		}
	}

	ref := remote.GetK8sRef()
	switch ref.GVK.GroupKind() {
	case schema.GroupKind{Group: "apps", Kind: "Deployment"}:
		fallthrough
	case schema.GroupKind{Group: "apps", Kind: "StatefulSet"}:
		ret = ret.Clone()
		err := ret.SetNestedField(0, "spec", "replicas")
		if err != nil {
			return nil, err
		}
	case schema.GroupKind{Group: "batch", Kind: "CronJob"}:
		ret = ret.Clone()
		err := ret.SetNestedField(true, "spec", "suspend")
		if err != nil {
			return nil, err
		}
	}

	return ret, nil
}
