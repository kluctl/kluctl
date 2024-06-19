package diff

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"regexp"
	"strings"
)

func listToMap(l []*uo.UnstructuredObject, key string) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	for _, e := range l {
		kv, found, _ := e.GetNestedString(key)
		if !found {
			return nil, fmt.Errorf("%s not found in list element", key)
		}
		m[kv] = e.Object
	}
	return m, nil
}

func normalizeEnv(container *uo.UnstructuredObject) {
	env := container.GetNestedObjectListNoErr("env")
	envFrom := container.GetNestedObjectListNoErr("envFrom")

	if len(env) != 0 {
		newEnv, err := listToMap(env, "name")
		if err == nil {
			_ = container.SetNestedField(newEnv, "env")
		}
	}
	if len(envFrom) != 0 {
		envTypes := []string{"configMapRef", "secretRef"}
		m := make(map[string]interface{})
		for _, e := range envFrom {
			k := ""
			for _, t := range envTypes {
				name, found, _ := e.GetNestedString(t, "name")
				if !found {
					continue
				}
				k = fmt.Sprintf("%s/%s", t, name)
			}
			if k == "" {
				if _, ok := m["unknown"]; !ok {
					m["unknown"] = []interface{}{}
				}
				m["unknown"] = append(m["unknown"].([]interface{}), e)
			} else {
				m[k] = e
			}
		}
		_ = container.SetNestedField(m, "envFrom")
	}
}

func normalizeContainers(containers []*uo.UnstructuredObject) {
	for _, c := range containers {
		normalizeEnv(c)
	}
}

func normalizeSecretAndConfigMaps(o *uo.UnstructuredObject) {
	data, found, _ := o.GetNestedObject("data")
	if found && (data == nil || len(data.Object) == 0) {
		_ = o.RemoveNestedField("data")
	}
}

func normalizeServiceAccount(o *uo.UnstructuredObject) {
	serviceAccountName, found, _ := o.GetNestedString("metadata", "name")
	if !found {
		return
	}

	secrets, found, _ := o.GetNestedObjectList("secrets")
	if !found {
		return
	}

	// remove default service account tokens
	var newSecrets []interface{}
	for _, s := range secrets {
		name, found, _ := s.GetNestedString("name")
		if !found || strings.HasPrefix(name, fmt.Sprintf("%s-", serviceAccountName)) {
			continue
		}
		newSecrets = append(newSecrets, s)
	}
	_ = o.SetNestedField(newSecrets, "secrets")
}

func normalizeMetadata(o *uo.UnstructuredObject) {
	// We don't care about managedFields when diffing (they just produce noise)
	_ = o.RemoveNestedField("metadata", "managedFields")
	_ = o.RemoveNestedField("metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration")

	// We don't want to see this in diffs
	_ = o.RemoveNestedField("metadata", "creationTimestamp")
	_ = o.RemoveNestedField("metadata", "generation")
	_ = o.RemoveNestedField("metadata", "resourceVersion")
	_ = o.RemoveNestedField("metadata", "selfLink")
	_ = o.RemoveNestedField("metadata", "uid")

	// Ensure empty labels/metadata exist
	_ = o.SetNestedFieldDefault(map[string]any{}, "metadata", "labels")
	_ = o.SetNestedFieldDefault(map[string]any{}, "metadata", "annotations")
}

func normalizeMisc(o *uo.UnstructuredObject) {
	// See https://kubernetes.io/docs/reference/labels-annotations-taints/#controller-uid
	_ = o.RemoveNestedField("spec", "template", "metadata", "labels", "controller-uid")
	_ = o.RemoveNestedField("spec", "selector", "matchLabels", "controller-uid")

	// See https://kubernetes.io/docs/reference/labels-annotations-taints/#batchkubernetesio-controller-uid
	_ = o.RemoveNestedField("spec", "template", "metadata", "labels", "batch.kubernetes.io/controller-uid")
	_ = o.RemoveNestedField("spec", "selector", "matchLabels", "batch.kubernetes.io/controller-uid")

	_ = o.RemoveNestedField("status")
}

func normalizeFloats(o *uo.UnstructuredObject) {
	_ = o.NewIterator().IterateLeafs(func(it *uo.ObjectIterator) error {
		if f, ok := it.Value().(float64); ok {
			i := int64(f)
			if f == float64(i) {
				_ = it.SetValue(i)
			}
		}
		return nil
	})
}

var ignoreDiffFieldAnnotationRegex = regexp.MustCompile(`^kluctl.io/ignore-diff-field(-\d*)?$`)
var ignoreDiffFieldRegexAnnotationRegex = regexp.MustCompile(`^kluctl.io/ignore-diff-field-regex(-\d*)?$`)

// NormalizeObject Performs some deterministic sorting and other normalizations to avoid ugly diffs due to order changes
func NormalizeObject(o_ *uo.UnstructuredObject, ignoreForDiffs []types.IgnoreForDiffItemConfig, localObject *uo.UnstructuredObject) (*uo.UnstructuredObject, error) {
	gvk := o_.GetK8sGVK()
	name := o_.GetK8sName()
	ns := o_.GetK8sNamespace()

	o := o_.Clone()
	normalizeFloats(o)
	normalizeMetadata(o)
	normalizeMisc(o)

	switch gvk.Kind {
	case "Deployment", "StatefulSet", "DaemonSet", "job":
		normalizeContainers(o.GetNestedObjectListNoErr("spec", "template", "spec", "containers"))
	case "Secret", "ConfigMap":
		normalizeSecretAndConfigMaps(o)
	case "ServiceAccount":
		normalizeServiceAccount(o)
	}

	if localObject.GetK8sAnnotationBoolNoError("kluctl.io/ignore-diff", false) {
		// Return empty object so that diffs will always be empty
		return &uo.UnstructuredObject{Object: map[string]interface{}{}}, nil
	}

	checkMatch := func(v string, m *string) bool {
		if v == "" || m == nil {
			return true
		}
		return v == *m
	}

	ignoreForDiffs = append([]types.IgnoreForDiffItemConfig{}, ignoreForDiffs...)
	for _, v := range localObject.GetK8sAnnotationsWithRegex(ignoreDiffFieldAnnotationRegex) {
		ignoreForDiffs = append(ignoreForDiffs, types.IgnoreForDiffItemConfig{
			FieldPath: []string{v},
		})
	}
	for _, v := range localObject.GetK8sAnnotationsWithRegex(ignoreDiffFieldRegexAnnotationRegex) {
		ignoreForDiffs = append(ignoreForDiffs, types.IgnoreForDiffItemConfig{
			FieldPathRegex: []string{v},
		})
	}

	for _, ifd := range ignoreForDiffs {
		if !checkMatch(gvk.Group, ifd.Group) {
			continue
		}
		if !checkMatch(gvk.Kind, ifd.Kind) {
			continue
		}
		if !checkMatch(ns, ifd.Namespace) {
			continue
		}
		if !checkMatch(name, ifd.Name) {
			continue
		}

		for _, fp := range ifd.FieldPath {
			jp, err := uo.NewMyJsonPath(fp)
			if err != nil {
				return nil, err
			}
			_ = jp.Del(o)
		}
		for _, fp := range ifd.FieldPathRegex {
			err := o.RemoveFieldsByPathRegex(fp)
			if err != nil {
				return nil, err
			}
		}
	}

	return o, nil
}
