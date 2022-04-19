package k8s

import (
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"k8s.io/apimachinery/pkg/runtime/schema"
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
