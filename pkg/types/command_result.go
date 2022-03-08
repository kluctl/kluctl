package types

import (
	"github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/utils/uo"
)

type Change struct {
	Type        string      `yaml:"type" validate:"required"`
	JsonPath    string      `yaml:"jsonPath" validate:"required"`
	OldValue    interface{} `yaml:"oldValue,omitempty"`
	NewValue    interface{} `yaml:"newValue,omitempty"`
	UnifiedDiff string      `yaml:"unifiedDiff,omitempty"`
}

type ChangedObject struct {
	Ref       k8s.ObjectRef          `yaml:"ref"`
	NewObject *uo.UnstructuredObject `yaml:"newObject,omitempty"`
	OldObject *uo.UnstructuredObject `yaml:"oldObject,omitempty"`
	Changes   []Change               `yaml:"changes,omitempty"`
}

type RefAndObject struct {
	Ref    k8s.ObjectRef          `yaml:"ref"`
	Object *uo.UnstructuredObject `yaml:"object,omitempty"`
}

type DeploymentError struct {
	Ref   k8s.ObjectRef `yaml:"ref"`
	Error string        `yaml:"error"`
}

type CommandResult struct {
	NewObjects     []*RefAndObject   `yaml:"newObjects,omitempty"`
	ChangedObjects []*ChangedObject  `yaml:"changedObjects,omitempty"`
	HookObjects    []*RefAndObject   `yaml:"hookObjects,omitempty"`
	OrphanObjects  []k8s.ObjectRef   `yaml:"orphanObjects,omitempty"`
	DeletedObjects []k8s.ObjectRef   `yaml:"deletedObjects,omitempty"`
	Errors         []DeploymentError `yaml:"errors,omitempty"`
	Warnings       []DeploymentError `yaml:"warnings,omitempty"`
	SeenImages     []FixedImage      `yaml:"seenImages,omitempty"`
}

type ValidateResultEntry struct {
	Ref        k8s.ObjectRef `yaml:"ref"`
	Annotation string        `yaml:"annotation"`
	Message    string        `yaml:"message"`
}

type ValidateResult struct {
	Ready    bool                  `yaml:"ready"`
	Warnings []DeploymentError     `yaml:"warnings,omitempty"`
	Errors   []DeploymentError     `yaml:"errors,omitempty"`
	Results  []ValidateResultEntry `yaml:"results,omitempty"`
}
