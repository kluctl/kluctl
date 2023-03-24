package k8s

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ObjectRef struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

func (r ObjectRef) String() string {
	if r.Namespace != "" {
		return fmt.Sprintf("%s/%s/%s", r.Namespace, r.Kind, r.Name)
	} else {
		if r.Name != "" {
			return fmt.Sprintf("%s/%s", r.Kind, r.Name)
		} else {
			return r.Kind
		}
	}
}

func (r ObjectRef) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   r.Group,
		Version: r.Version,
		Kind:    r.Kind,
	}
}

func (r ObjectRef) GroupKind() schema.GroupKind {
	return schema.GroupKind{
		Group: r.Group,
		Kind:  r.Kind,
	}
}

func (r ObjectRef) GroupVersion() schema.GroupVersion {
	return schema.GroupVersion{
		Group:   r.Group,
		Version: r.Version,
	}
}

func NewObjectRef(group string, version string, kind string, name string, namespace string) ObjectRef {
	return ObjectRef{
		Group:     group,
		Version:   version,
		Kind:      kind,
		Name:      name,
		Namespace: namespace,
	}
}
