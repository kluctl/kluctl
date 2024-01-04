// +kubebuilder:object:generate=true
package k8s

import (
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type ObjectRef struct {
	Group     string `json:"group,omitempty"`
	Version   string `json:"version,omitempty"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
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

func (r ObjectRef) Less(o ObjectRef) bool {
	if r.Group != o.Group {
		return r.Group < o.Group
	} else if r.Version != o.Version {
		return r.Version < o.Version
	} else if r.Kind != o.Kind {
		return r.Kind < o.Kind
	} else if r.Namespace != o.Namespace {
		return r.Namespace < o.Namespace
	} else {
		return r.Name < o.Name
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
