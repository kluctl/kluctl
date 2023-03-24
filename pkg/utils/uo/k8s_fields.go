package uo

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"reflect"
	"regexp"
	"time"
)

func (uo *UnstructuredObject) GetK8sGVK() schema.GroupVersionKind {
	kind, _, err := uo.GetNestedString("kind")
	if err != nil {
		panic(err)
	}
	apiVersion, _, err := uo.GetNestedString("apiVersion")
	if err != nil {
		panic(err)
	}
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		panic(err)
	}
	return schema.GroupVersionKind{
		Group:   gv.Group,
		Version: gv.Version,
		Kind:    kind,
	}
}

func (uo *UnstructuredObject) SetK8sGVK(gvk schema.GroupVersionKind) {
	err := uo.SetNestedField(gvk.GroupVersion().String(), "apiVersion")
	if err != nil {
		panic(err)
	}
	err = uo.SetNestedField(gvk.Kind, "kind")
	if err != nil {
		panic(err)
	}
}

func (uo *UnstructuredObject) SetK8sGVKs(g string, v string, k string) {
	uo.SetK8sGVK(schema.GroupVersionKind{Group: g, Version: v, Kind: k})
}

func (uo *UnstructuredObject) GetK8sName() string {
	s, _, err := uo.GetNestedString("metadata", "name")
	if err != nil {
		panic(err)
	}
	return s
}

func (uo *UnstructuredObject) SetK8sName(name string) {
	err := uo.SetNestedField(name, "metadata", "name")
	if err != nil {
		panic(err)
	}
}

func (uo *UnstructuredObject) GetK8sNamespace() string {
	s, _, err := uo.GetNestedString("metadata", "namespace")
	if err != nil {
		panic(err)
	}
	return s
}

func (uo *UnstructuredObject) SetK8sNamespace(namespace string) {
	if namespace != "" {
		err := uo.SetNestedField(namespace, "metadata", "namespace")
		if err != nil {
			panic(err)
		}
	} else {
		err := uo.RemoveNestedField("metadata", "namespace")
		if err != nil {
			panic(err)
		}
	}

}

func (uo *UnstructuredObject) GetK8sRef() k8s.ObjectRef {
	gvk := uo.GetK8sGVK()
	return k8s.ObjectRef{
		Group:     gvk.Group,
		Version:   gvk.Version,
		Kind:      gvk.Kind,
		Name:      uo.GetK8sName(),
		Namespace: uo.GetK8sNamespace(),
	}
}

func (uo *UnstructuredObject) GetK8sLabels() map[string]string {
	ret, ok, err := uo.GetNestedStringMapCopy("metadata", "labels")
	if err != nil {
		panic(err)
	}
	if !ok {
		return map[string]string{}
	}
	return ret
}

func (uo *UnstructuredObject) SetK8sLabels(labels map[string]string) {
	_ = uo.RemoveNestedField("metadata", "labels")
	for k, v := range labels {
		uo.SetK8sLabel(k, v)
	}
}

func (uo *UnstructuredObject) GetK8sLabel(name string) *string {
	ret, ok, err := uo.GetNestedString("metadata", "labels", name)
	if err != nil {
		panic(err)
	}
	if !ok {
		return nil
	}
	return &ret
}

func (uo *UnstructuredObject) SetK8sLabel(name string, value string) {
	err := uo.SetNestedField(value, "metadata", "labels", name)
	if err != nil {
		panic(err)
	}
}

func (uo *UnstructuredObject) GetK8sLabelsWithRegex(r interface{}) map[string]string {
	p := uo.getRegexp(r)

	ret := make(map[string]string)
	for k, v := range uo.GetK8sLabels() {
		if p.MatchString(k) {
			ret[k] = v
		}
	}
	return ret
}

func (uo *UnstructuredObject) GetK8sAnnotations() map[string]string {
	ret, ok, err := uo.GetNestedStringMapCopy("metadata", "annotations")
	if err != nil {
		panic(err)
	}
	if !ok {
		return map[string]string{}
	}
	return ret
}

func (uo *UnstructuredObject) GetK8sAnnotation(name string) *string {
	ret, ok, err := uo.GetNestedString("metadata", "annotations", name)
	if err != nil {
		panic(err)
	}
	if !ok {
		return nil
	}
	return &ret
}

func (uo *UnstructuredObject) SetK8sAnnotations(annotations map[string]string) {
	_ = uo.RemoveNestedField("metadata", "annotations")
	for k, v := range annotations {
		uo.SetK8sAnnotation(k, v)
	}
}

func (uo *UnstructuredObject) SetK8sAnnotation(name string, value string) {
	err := uo.SetNestedField(value, "metadata", "annotations", name)
	if err != nil {
		panic(err)
	}
}

func (uo *UnstructuredObject) GetK8sAnnotationsWithRegex(r interface{}) map[string]string {
	p := uo.getRegexp(r)

	ret := make(map[string]string)
	for k, v := range uo.GetK8sAnnotations() {
		if p.MatchString(k) {
			ret[k] = v
		}
	}
	return ret
}

func (uo *UnstructuredObject) GetK8sGeneration() int64 {
	ret, ok, _ := uo.GetNestedInt("metadata", "generation")
	if !ok {
		return -1
	}
	return ret
}

func (uo *UnstructuredObject) GetK8sResourceVersion() string {
	ret, _, _ := uo.GetNestedString("metadata", "resourceVersion")
	return ret
}

func (uo *UnstructuredObject) SetK8sResourceVersion(rv string) {
	if rv == "" {
		_ = uo.RemoveNestedField("metadata", "resourceVersion")
	} else {
		err := uo.SetNestedField(rv, "metadata", "resourceVersion")
		if err != nil {
			panic(err)
		}
	}
}

func (uo *UnstructuredObject) GetK8sOwnerReferences() []*UnstructuredObject {
	ret, _, _ := uo.GetNestedObjectList("metadata", "ownerReferences")
	return ret
}

func (uo *UnstructuredObject) GetK8sManagedFields() []*UnstructuredObject {
	ret, _, _ := uo.GetNestedObjectList("metadata", "managedFields")
	return ret
}

func (uo *UnstructuredObject) GetK8sCreationTime() time.Time {
	v, ok, _ := uo.GetNestedString("metadata", "creationTimestamp")
	if !ok {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (ui *UnstructuredObject) getRegexp(r interface{}) *regexp.Regexp {
	if x, ok := r.(*regexp.Regexp); ok {
		return x
	} else {
		if x, ok := r.(string); ok {
			return regexp.MustCompile(x)
		}
	}
	panic(fmt.Sprintf("unknown type %s", reflect.TypeOf(r).String()))
	return nil
}
