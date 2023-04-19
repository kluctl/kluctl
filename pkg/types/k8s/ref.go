package k8s

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ObjectRef struct {
	GVK       schema.GroupVersionKind `json:"gvk,inline"`
	Name      string
	Namespace string
}

func (r ObjectRef) String() string {
	if r.Namespace != "" {
		return fmt.Sprintf("%s/%s/%s", r.Namespace, r.GVK.Kind, r.Name)
	} else {
		if r.Name != "" {
			return fmt.Sprintf("%s/%s", r.GVK.Kind, r.Name)
		} else {
			return r.GVK.Kind
		}
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
