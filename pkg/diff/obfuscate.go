package diff

import (
	"encoding/base64"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/ohler55/ojg/jp"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
)

var secretGk = schema.GroupKind{Group: "", Kind: "Secret"}

type Obfuscator struct {
}

func (o *Obfuscator) ObfuscateChanges(ref k8s.ObjectRef, changes []result.Change) error {
	if ref.GVK.GroupKind() == secretGk {
		err := o.obfuscateSecretChanges(ref, changes)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *Obfuscator) ObfuscateObject(x *uo.UnstructuredObject) error {
	ref := x.GetK8sRef()
	if ref.GVK.GroupKind() == secretGk {
		err := o.obfuscateSecret(x)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o *Obfuscator) obfuscateSecretChanges(ref k8s.ObjectRef, changes []result.Change) error {
	replaceValues := func(x any, v string) any {
		if x == nil {
			return nil
		}
		if m, ok := x.(map[string]any); ok {
			for k, _ := range m {
				m[k] = v
			}
			return m
		} else if a, ok := x.([]any); ok {
			for i, _ := range a {
				a[i] = v
			}
			return a
		}
		return v
	}

	for i, _ := range changes {
		c := &changes[i]
		j, err := jp.ParseString(c.JsonPath)
		if err != nil {
			return err
		}
		if len(j) == 0 {
			return fmt.Errorf("unexpected empty jsonPath")
		}
		child, ok := j[0].(jp.Child)
		if !ok {
			return fmt.Errorf("unexpected jsonPath fragment: %s", c.JsonPath)
		}

		if child == "data" || child == "stringData" {
			c.NewValue = replaceValues(c.NewValue, "*****a")
			c.OldValue = replaceValues(c.OldValue, "*****b")
			_ = updateUnifiedDiff(c)
			c.NewValue = replaceValues(c.NewValue, "*****")
			c.OldValue = replaceValues(c.OldValue, "*****")
			c.UnifiedDiff = strings.ReplaceAll(c.UnifiedDiff, "*****a", "***** (obfuscated)")
			c.UnifiedDiff = strings.ReplaceAll(c.UnifiedDiff, "*****b", "***** (obfuscated)")
		}
	}
	return nil
}

func (o *Obfuscator) obfuscateSecret(x *uo.UnstructuredObject) error {
	data, ok, _ := x.GetNestedField("data")
	if ok {
		if m, ok := data.(map[string]any); ok {
			for k, _ := range m {
				m[k] = base64.StdEncoding.EncodeToString([]byte("*****"))
			}
		} else {
			return fmt.Errorf("'data' is not a map of strings")
		}
	}
	data, ok, _ = x.GetNestedField("stringData")
	if ok {
		if m, ok := data.(map[string]any); ok {
			for k, _ := range m {
				m[k] = "*****"
			}
		} else {
			return fmt.Errorf("'data' is not a map of strings")
		}
	}
	return nil
}
