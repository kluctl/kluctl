package types

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Change struct {
	Type        string      `yaml:"type" validate:"required"`
	JsonPath    string      `yaml:"jsonPath" validate:"required"`
	OldValue    interface{} `yaml:"oldValue,omitempty"`
	NewValue    interface{} `yaml:"newValue,omitempty"`
	UnifiedDiff string      `yaml:"unifiedDiff,omitempty"`
}

type ChangedObject struct {
	NewObject *unstructured.Unstructured `yaml:"newObject,omitempty"`
	OldObject *unstructured.Unstructured `yaml:"oldObject,omitempty"`
	Changes   []Change                   `yaml:"changes,omitempty"`
}

type DeploymentError struct {
	Ref   ObjectRef `yaml:"ref"`
	Error string    `yaml:"error"`
}

type CommandResult struct {
	NewObjects     []*unstructured.Unstructured `yaml:"newObjects,omitempty"`
	ChangedObjects []*ChangedObject             `yaml:"changedObjects,omitempty"`
	HookObjects    []*unstructured.Unstructured `yaml:"hook-objects,omitempty"`
	OrphanObjects  []ObjectRef                  `yaml:"orphanObjects,omitempty"`
	DeletedObjects []ObjectRef                  `yaml:"deletedObjects,omitempty"`
	Errors         []DeploymentError            `yaml:"errors,omitempty"`
	Warnings       []DeploymentError            `yaml:"warnings,omitempty"`
	SeenImages     []FixedImage                 `yaml:"seenImages,omitempty"`
}

type ValidateResultEntry struct {
	Ref        ObjectRef `yaml:"ref"`
	Annotation string    `yaml:"annotation"`
	Message    string    `yaml:"message"`
}

type ValidateResult struct {
	Ready    bool                  `yaml:"ref"`
	Warnings []DeploymentError     `yaml:"warnings,omitempty"`
	Errors   []DeploymentError     `yaml:"errors,omitempty"`
	Results  []ValidateResultEntry `yaml:"results,omitempty"`
}
