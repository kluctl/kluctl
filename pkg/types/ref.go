package types

import (
	"fmt"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ObjectRef struct {
	GVK       schema.GroupVersionKind `yaml:"gvk,inline"`
	Name      string
	Namespace string
}

func (r ObjectRef) String() string {
	if r.Namespace != "" {
		return fmt.Sprintf("%s/%s/%s", r.Namespace, r.GVK.Kind, r.Name)
	} else {
		return fmt.Sprintf("%s/%s", r.GVK.Kind, r.Name)
	}
}

func NewObjectRef(group string, version string, kind string, name string, namespace string) ObjectRef {
	return ObjectRef{
		GVK: schema.GroupVersionKind{
			Group:   group,
			Version: version,
			Kind:    kind,
		},
		Name:      name,
		Namespace: namespace,
	}
}

func RefFromObject(o *unstructured.Unstructured) ObjectRef {
	return ObjectRef{
		GVK:       o.GroupVersionKind(),
		Name:      o.GetName(),
		Namespace: o.GetNamespace(),
	}
}

func RefFromPartialObject(o *v1.PartialObjectMetadata) ObjectRef {
	return ObjectRef{
		GVK:       o.GroupVersionKind(),
		Name:      o.GetName(),
		Namespace: o.GetNamespace(),
	}
}
