package diff

import (
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strings"
)

var secretGvk = schema.GroupKind{Group: "", Kind: "Secret"}

type Obfuscator struct {
}

func (o *Obfuscator) Obfuscate(ref k8s.ObjectRef, changes []types.Change) {
	if ref.GVK.GroupKind() == secretGvk {
		o.obfuscateSecret(ref, changes)
	}
}

func (o *Obfuscator) obfuscateSecret(ref k8s.ObjectRef, changes []types.Change) {
	for i, _ := range changes {
		c := &changes[i]
		if strings.HasPrefix(c.JsonPath, "data.") || strings.HasPrefix(c.JsonPath, "stringData.") {
			if c.NewValue != nil {
				c.NewValue = "*****a"
			}
			if c.OldValue != nil {
				c.OldValue = "*****b"
			}
			_ = updateUnifiedDiff(c)
			if c.NewValue != nil {
				c.NewValue = "*****"
				c.UnifiedDiff = strings.ReplaceAll(c.UnifiedDiff, "*****a", "***** (obfuscated)")
			}
			if c.OldValue != nil {
				c.OldValue = "*****"
				c.UnifiedDiff = strings.ReplaceAll(c.UnifiedDiff, "*****b", "***** (obfuscated)")
			}
		}
	}
}
