package validation

import (
	"github.com/codablock/kluctl/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type ValidateResultItem struct {
	Ref     types.ObjectRef `yaml:"ref"`
	Reason  string          `yaml:"reason"`
	Message string          `yaml:"message"`
}

type ValidationResult struct {
	Ready    bool                 `yaml:"ready"`
	Warnings []ValidateResultItem `yaml:"warnings,omitempty"`
	Errors   []ValidateResultItem `yaml:"errors,omitempty"`
	Results  []ValidateResultItem `yaml:"results,omitempty"`
}

func ValidateObject(o *unstructured.Unstructured, notReadyIsError bool) ValidationResult {
	return ValidationResult{
		Ready: false,
	}
}
