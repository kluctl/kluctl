package k8s

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func UnwrapListItems(o *uo.UnstructuredObject, withListCallback bool, cb func(o *uo.UnstructuredObject) error) error {
	listGvk := schema.GroupVersionKind{Group: "", Version: "v1", Kind: "List"}
	if o.GetK8sGVK() == listGvk {
		if withListCallback {
			err := cb(o)
			if err != nil {
				return err
			}
		}
		items, _, err := o.GetNestedObjectList("items")
		if err != nil {
			return err
		}
		for _, x := range items {
			err = cb(x)
			if err != nil {
				return err
			}
		}
		return nil
	} else {
		return cb(o)
	}
}

func FixNamespace(o *uo.UnstructuredObject, namespaced bool, def string) {
	ref := o.GetK8sRef()
	if !namespaced && ref.Namespace != "" {
		o.SetK8sNamespace("")
	} else if namespaced && ref.Namespace == "" {
		o.SetK8sNamespace(def)
	}
}

func FixNamespaceInRef(ref k8s.ObjectRef, namespaced bool, def string) k8s.ObjectRef {
	if !namespaced && ref.Namespace != "" {
		ref.Namespace = ""
	} else if namespaced && ref.Namespace == "" {
		ref.Namespace = def
	}
	return ref
}

func GetClusterId(ctx context.Context, c client.Client) (string, error) {
	// we reuse the kube-system namespace uid as global cluster id
	var ns corev1.Namespace
	err := c.Get(ctx, client.ObjectKey{Name: "kube-system"}, &ns)
	if err != nil {
		return "", err
	}
	clusterId := ns.UID
	if clusterId == "" {
		return "", fmt.Errorf("kube-system namespace has no uid")
	}
	return string(clusterId), nil
}
