package diff

import (
	"fmt"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"
)

// based on sigs.k8s.io/structured-merge-diff/v4/fieldpath/serialize.go:readIterV1
func convertManagedFields(m map[string]any) (children *fieldpath.Set, isMember bool, retErr error) {
	for key, v := range m {
		if key == "." {
			isMember = true
			continue
		}
		pe, err := fieldpath.DeserializePathElement(key)
		if err == fieldpath.ErrUnknownPathElementType {
			// Ignore these-- a future version maybe knows what
			// they are. We drop these completely rather than try
			// to preserve things we don't understand.
			continue
		} else if err != nil {
			retErr = fmt.Errorf("parsing key as path element: %w", err)
			return
		}
		m2, ok := v.(map[string]any)
		if !ok {
			retErr = fmt.Errorf("value is not a map")
			return
		}
		grandchildren, childIsMember, err := convertManagedFields(m2)
		if err != nil {
			retErr = err
			return
		}
		if childIsMember {
			if children == nil {
				children = &fieldpath.Set{}
			}

			children.Members.Insert(pe)
		}
		if grandchildren != nil {
			if children == nil {
				children = &fieldpath.Set{}
			}
			*children.Children.Descend(pe) = *grandchildren
		}
	}
	if children == nil {
		isMember = true
	}

	return children, isMember, nil
}
