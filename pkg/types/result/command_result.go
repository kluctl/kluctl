package result

import (
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
)

type Change struct {
	Type        string      `json:"type" validate:"required"`
	JsonPath    string      `json:"jsonPath" validate:"required"`
	OldValue    interface{} `json:"oldValue,omitempty"`
	NewValue    interface{} `json:"newValue,omitempty"`
	UnifiedDiff string      `json:"unifiedDiff,omitempty"`
}

type ChangedObject struct {
	Ref       k8s.ObjectRef          `json:"ref"`
	NewObject *uo.UnstructuredObject `json:"newObject,omitempty"`
	OldObject *uo.UnstructuredObject `json:"oldObject,omitempty"`
	Changes   []Change               `json:"changes,omitempty"`
}

type RefAndObject struct {
	Ref    k8s.ObjectRef          `json:"ref"`
	Object *uo.UnstructuredObject `json:"object,omitempty"`
}

type DeploymentError struct {
	Ref   k8s.ObjectRef `json:"ref"`
	Error string        `json:"error"`
}

type CommandResult struct {
	NewObjects     []*RefAndObject    `json:"newObjects,omitempty"`
	ChangedObjects []*ChangedObject   `json:"changedObjects,omitempty"`
	HookObjects    []*RefAndObject    `json:"hookObjects,omitempty"`
	OrphanObjects  []k8s.ObjectRef    `json:"orphanObjects,omitempty"`
	DeletedObjects []k8s.ObjectRef    `json:"deletedObjects,omitempty"`
	Errors         []DeploymentError  `json:"errors,omitempty"`
	Warnings       []DeploymentError  `json:"warnings,omitempty"`
	SeenImages     []types.FixedImage `json:"seenImages,omitempty"`
}

type ValidateResultEntry struct {
	Ref        k8s.ObjectRef `json:"ref"`
	Annotation string        `json:"annotation"`
	Message    string        `json:"message"`
}

type ValidateResult struct {
	Ready    bool                  `json:"ready"`
	Warnings []DeploymentError     `json:"warnings,omitempty"`
	Errors   []DeploymentError     `json:"errors,omitempty"`
	Results  []ValidateResultEntry `json:"results,omitempty"`
}
