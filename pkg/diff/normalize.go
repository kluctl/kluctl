package diff

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"regexp"
	"strconv"
	"strings"
)

func listToMap(l []map[string]interface{}, key string) (map[string]interface{}, error) {
	m := make(map[string]interface{})
	for _, e := range l {
		kv, ok := e[key]
		if !ok {
			return nil, fmt.Errorf("%s not found in list element", key)
		}
		kvs, ok := kv.(string)
		if !ok {
			return nil, fmt.Errorf("%s in list element is not a string", key)
		}
		m[kvs] = e
	}
	return m, nil
}

func normalizeEnv(container map[string]interface{}) {
	env := utils.NestedMapSliceNoErr(container, "env")
	envFrom := utils.NestedMapSliceNoErr(container, "envFrom")

	if len(env) != 0 {
		newEnv, err := listToMap(env, "name")
		if err == nil {
			container["env"] = newEnv
		}
	}
	if len(envFrom) != 0 {
		envTypes := []string{"configMapRef", "secretRef"}
		m := make(map[string]interface{})
		for _, e := range envFrom {
			k := ""
			for _, t := range envTypes {
				name, found, _ := unstructured.NestedString(e, t, "name")
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
		container["envFrom"] = m
	}
}

func normalizeContainers(containers []map[string]interface{}) {
	for _, c := range containers {
		normalizeEnv(c)
	}
}

func normalizeSecretAndConfigMaps(o map[string]interface{}) {
	data, found, _ := unstructured.NestedMap(o, "data")
	if found && len(data) == 0 {
		unstructured.RemoveNestedField(o, "data")
	}
}

func normalizeServiceAccount(o map[string]interface{}) {
	serviceAccountName, found, _ := unstructured.NestedString(o, "metadata", "name")
	if !found {
		return
	}

	secrets, found, _ := unstructured.NestedSlice(o, "secrets")
	if !found {
		return
	}

	// remove default service account tokens
	var newSecrets []interface{}
	for _, s := range secrets {
		s2, ok := s.(map[string]interface{})
		if !ok {
			continue
		}

		name, found, _ := unstructured.NestedString(s2, "name")
		if !found || strings.HasPrefix(name, fmt.Sprintf("%s-", serviceAccountName)) {
			continue
		}
		newSecrets = append(newSecrets, s)
	}
	o["secrets"] = newSecrets
}

func normalizeMetadata(k *k8s.K8sCluster, o *unstructured.Unstructured) {
	k.RemoveNamespaceIfNeeded(o)

	// We don't care about managedFields when diffing (they just produce noise)
	unstructured.RemoveNestedField(o.Object, "metadata", "managedFields")
	unstructured.RemoveNestedField(o.Object, "metadata", "annotations", "managedFields", "kubectl.kubernetes.io/last-applied-configuration")

	// We don't want to see this in diffs
	unstructured.RemoveNestedField(o.Object, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(o.Object, "metadata", "generation")
	unstructured.RemoveNestedField(o.Object, "metadata", "resourceVersion")
	unstructured.RemoveNestedField(o.Object, "metadata", "selfLink")
	unstructured.RemoveNestedField(o.Object, "metadata", "uid")

	// Ensure empty labels/metadata exist
	if len(o.GetLabels()) == 0 {
		_ = unstructured.SetNestedStringMap(o.Object, map[string]string{}, "metadata", "labels")
	}
	if len(o.GetAnnotations()) == 0 {
		_ = unstructured.SetNestedStringMap(o.Object, map[string]string{}, "metadata", "annotations")
	}
}

func normalizeMisc(o map[string]interface{}) {
	// These are random values found in Jobs
	unstructured.RemoveNestedField(o, "spec", "template", "metadata", "labels", "controller-uid")
	unstructured.RemoveNestedField(o, "spec", "selector", "matchLabels", "controller-uid")

	unstructured.RemoveNestedField(o, "status")
}

var ignoreDiffFieldAnnotationRegex = regexp.MustCompile(`^kluctl.io/ignore-diff-field(-\d*)?$`)

// NormalizeObject Performs some deterministic sorting and other normalizations to avoid ugly diffs due to order changes
func NormalizeObject(k *k8s.K8sCluster, o *unstructured.Unstructured, ignoreForDiffs []*types.IgnoreForDiffItemConfig, localObject *unstructured.Unstructured) *unstructured.Unstructured {
	gvk := o.GroupVersionKind()
	name := o.GetName()
	ns := o.GetNamespace()

	o = utils.CopyUnstructured(o)
	normalizeMetadata(k, o)
	normalizeMisc(o.Object)

	switch gvk.Kind {
	case "Deployment", "StatefulSet", "DaemonSet", "job":
		normalizeContainers(utils.NestedMapSliceNoErr(o.Object, "spec", "template", "spec", "containers"))
	case "Secret", "ConfigMap":
		normalizeSecretAndConfigMaps(o.Object)
	case "ServiceAccount":
		normalizeServiceAccount(o.Object)
	}

	checkMatch := func(v string, m *string) bool {
		if v == "" || m == nil {
			return true
		}
		return v == *m
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
			jp, err := utils.NewMyJsonPath(fp)
			if err != nil {
				continue
			}
			_ = jp.Del(o.Object)
		}
	}

	ignoreAll, _ := strconv.ParseBool(localObject.GetAnnotations()["kluctl.io/ignore-diff"])
	if ignoreAll {
		// Return empty object so that diffs will always be empty
		return &unstructured.Unstructured{Object: map[string]interface{}{}}
	}

	for k, v := range localObject.GetAnnotations() {
		if ignoreDiffFieldAnnotationRegex.MatchString(k) {
			j, err := utils.NewMyJsonPath(v)
			if err != nil {
				continue
			}
			_ = j.Del(o.Object)
		}
	}

	return o
}
