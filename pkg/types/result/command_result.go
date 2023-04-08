package result

import (
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
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
	Ref     k8s.ObjectRef `json:"ref"`
	Message string        `json:"message"`
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

type ProjectKey struct {
	NormalizedGitUrl string `json:"normalizedGitUrl,omitempty"`
	SubDir           string `json:"subDir,omitempty"`
}

type CommandInfo struct {
	Initiator             CommandInitiator       `json:"initiator" validate:"oneof=CommandLine KluctlDeployment"`
	StartTime             types.JsonTime         `json:"startTime"`
	EndTime               types.JsonTime         `json:"endTime"`
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

type GitInfo struct {
	Url    *git_url.GitUrl `json:"url"`
	Ref    string          `json:"ref"`
	SubDir string          `json:"subDir"`
	Commit string          `json:"commit"`
	Dirty  bool            `json:"dirty"`
}

type BaseObject struct {
	Ref     k8s.ObjectRef `json:"ref"`
	Changes []Change      `json:"changes,omitempty"`

	New     bool `json:"new,omitempty"`
	Orphan  bool `json:"orphan,omitempty"`
	Deleted bool `json:"deleted,omitempty"`
	Hook    bool `json:"hook,omitempty"`
}

type ResultObject struct {
	BaseObject

	Rendered *uo.UnstructuredObject `json:"rendered,omitempty"`
	Remote   *uo.UnstructuredObject `json:"remote,omitempty"`
	Applied  *uo.UnstructuredObject `json:"applied,omitempty"`
}

type CommandResult struct {
	Id         string                         `json:"id"`
	Project    ProjectKey                     `json:"project"`
	Command    CommandInfo                    `json:"command,omitempty"`
	GitInfo    *GitInfo                       `json:"gitInfo,omitempty"`
	Deployment *types.DeploymentProjectConfig `json:"deployment,omitempty"`

	Objects []ResultObject `json:"objects,omitempty"`

	Errors     []DeploymentError  `json:"errors,omitempty"`
	Warnings   []DeploymentError  `json:"warnings,omitempty"`
	SeenImages []types.FixedImage `json:"seenImages,omitempty"`
}

func (cr *CommandResult) ToCompacted() *CompactedCommandResult {
	ret := &CompactedCommandResult{
		CommandResult: *cr,
	}
	ret.CompactedObjects = ret.Objects
	ret.Objects = nil
	return ret
}

func (cr *CommandResult) ToReducedObjects() *CommandResult {
	ret := *cr
	ret.Objects = make([]ResultObject, len(ret.Objects))
	for i, o := range cr.Objects {
		ret.Objects[i] = o
		ret.Objects[i].Rendered = buildReducedObject(o.Rendered)
		ret.Objects[i].Remote = buildReducedObject(o.Remote)
		ret.Objects[i].Applied = buildReducedObject(o.Applied)
	}
	return &ret
}

type CompactedCommandResult struct {
	CommandResult

	CompactedObjects CompactedObjects `json:"compactedObjects,omitempty"`
}

func (ccr *CompactedCommandResult) ToNonCompacted() *CommandResult {
	ret := ccr.CommandResult
	ret.Objects = ccr.CompactedObjects
	return &ret
}

type ValidateResultEntry struct {
	Ref        k8s.ObjectRef `json:"ref"`
	Annotation string        `json:"annotation"`
	Message    string        `json:"message"`
}

type ValidateResult struct {
	Id       string                `json:"id"`
	Ready    bool                  `json:"ready"`
	Warnings []DeploymentError     `json:"warnings,omitempty"`
	Errors   []DeploymentError     `json:"errors,omitempty"`
	Results  []ValidateResultEntry `json:"results,omitempty"`
}
