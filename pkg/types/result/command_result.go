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
	Ref     k8s.ObjectRef `json:"ref"`
	Changes []Change      `json:"changes,omitempty"`
}

type DeploymentError struct {
	Ref   k8s.ObjectRef `json:"ref"`
	Error string        `json:"error"`
}

type KluctlDeploymentInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	GitUrl    string `json:"gitUrl"`
	GitRef    string `json:"gitRef"`
}

type CommandInitiator string

const (
	CommandInititiator_CommandLine      CommandInitiator = "CommandLine"
	CommandInititiator_KluctlDeployment                  = "KluctlDeployment"
)

type CommandInfo struct {
	Initiator             CommandInitiator       `json:"initiator" validate:"oneof=CommandLine KluctlDeployment"`
	KluctlDeployment      *KluctlDeploymentInfo  `json:"kluctlDeployment,omitempty"`
	Command               string                 `json:"command,omitempty"`
	Target                *types.Target          `json:"target,omitempty"`
	TargetNameOverride    string                 `json:"targetNameOverride,omitempty"`
	ContextOverride       string                 `json:"contextOverride,omitempty"`
	Args                  *uo.UnstructuredObject `json:"args,omitempty"`
	Images                []types.FixedImage     `json:"images,omitempty"`
	DryRun                bool                   `json:"dryRun,omitempty"`
	NoWait                bool                   `json:"noWait,omitempty"`
	ForceApply            bool                   `json:"forceApply,omitempty"`
	ReplaceOnError        bool                   `json:"replaceOnError,omitempty"`
	ForceReplaceOnError   bool                   `json:"forceReplaceOnError,omitempty"`
	AbortOnError          bool                   `json:"abortOnError,omitempty"`
	IncludeTags           []string               `json:"includeTags,omitempty"`
	ExcludeTags           []string               `json:"excludeTags,omitempty"`
	IncludeDeploymentDirs []string               `json:"includeDeploymentDirs,omitempty"`
	ExcludeDeploymentDirs []string               `json:"excludeDeploymentDirs,omitempty"`
}

type CommandResult struct {
	Command         *CommandInfo                   `json:"command,omitempty"`
	Deployment      *types.DeploymentProjectConfig `json:"deployment,omitempty"`
	RenderedObjects []*uo.UnstructuredObject       `json:"renderedObjects,omitempty"`

	NewObjects     []*uo.UnstructuredObject `json:"newObjects,omitempty"`
	ChangedObjects []*ChangedObject         `json:"changedObjects,omitempty"`
	HookObjects    []*uo.UnstructuredObject `json:"hookObjects,omitempty"`
	OrphanObjects  []k8s.ObjectRef          `json:"orphanObjects,omitempty"`
	DeletedObjects []k8s.ObjectRef          `json:"deletedObjects,omitempty"`
	Errors         []DeploymentError        `json:"errors,omitempty"`
	Warnings       []DeploymentError        `json:"warnings,omitempty"`
	SeenImages     []types.FixedImage       `json:"seenImages,omitempty"`
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
