package validation

import (
	"github.com/codablock/kluctl/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ValidateObject(o *unstructured.Unstructured, notReadyIsError bool) types.ValidateResult {
	return types.ValidateResult{
		Ready: false,
	}
}
