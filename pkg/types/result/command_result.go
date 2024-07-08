package result

import (
	gittypes "github.com/kluctl/kluctl/v2/lib/git/types"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type Change struct {
	Type        string                `json:"type" validate:"required"`
	JsonPath    string                `json:"jsonPath" validate:"required"`
	OldValue    *apiextensionsv1.JSON `json:"oldValue,omitempty"`
	NewValue    *apiextensionsv1.JSON `json:"newValue,omitempty"`
	UnifiedDiff string                `json:"unifiedDiff,omitempty"`
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
	ClusterId string `json:"clusterId"`
}

type CommandInitiator string

const (
	CommandInititiator_CommandLine      CommandInitiator = "CommandLine"
	CommandInititiator_KluctlDeployment                  = "KluctlDeployment"
)

type ProjectKey struct {
	RepoKey gittypes.RepoKey `json:"repoKey,omitempty"`
	SubDir  string           `json:"subDir,omitempty"`
}

func (k ProjectKey) Less(o ProjectKey) bool {
	if k.RepoKey != o.RepoKey {
		return k.RepoKey.String() < o.RepoKey.String()
	}
	if k.SubDir != o.SubDir {
		return k.SubDir < o.SubDir
	}
	return false
}

type TargetKey struct {
	TargetName    string `json:"targetName,omitempty"`
	ClusterId     string `json:"clusterId"`
	Discriminator string `json:"discriminator,omitempty"`
}

func (k TargetKey) Less(o TargetKey) bool {
	if k.TargetName != o.TargetName {
		return k.TargetName < o.TargetName
	}
	if k.ClusterId != o.ClusterId {
		return k.ClusterId < o.ClusterId
	}
	if k.Discriminator != o.Discriminator {
		return k.Discriminator < o.Discriminator
	}
	return false
}

type CommandInfo struct {
	Initiator             CommandInitiator       `json:"initiator" validate:"oneof=CommandLine KluctlDeployment"`
	StartTime             metav1.Time            `json:"startTime"`
	EndTime               metav1.Time            `json:"endTime"`
	Command               string                 `json:"command,omitempty"`
	Target                string                 `json:"target,omitempty"`
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

type ClusterInfo struct {
	ClusterId string `json:"clusterId"`
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
	Id               string                         `json:"id"`
	ReconcileId      string                         `json:"reconcileId"`
	ProjectKey       ProjectKey                     `json:"projectKey"`
	TargetKey        TargetKey                      `json:"targetKey"`
	Target           types.Target                   `json:"target"`
	Command          CommandInfo                    `json:"command,omitempty"`
	KluctlDeployment *KluctlDeploymentInfo          `json:"kluctlDeployment,omitempty"`
	OverridesPatch   *uo.UnstructuredObject         `json:"overridesPatch,omitempty"`
	GitInfo          gittypes.GitInfo               `json:"gitInfo,omitempty"`
	ClusterInfo      ClusterInfo                    `json:"clusterInfo"`
	Deployment       *types.DeploymentProjectConfig `json:"deployment,omitempty"`

	RenderedObjectsHash string         `json:"renderedObjectsHash,omitempty"`
	Objects             []ResultObject `json:"objects,omitempty"`

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
		ret.Objects[i].Rendered = BuildReducedObject(o.Rendered)
		ret.Objects[i].Remote = BuildReducedObject(o.Remote)
		ret.Objects[i].Applied = BuildReducedObject(o.Applied)
	}
	return &ret
}

type CompactedCommandResult struct {
	CommandResult

	CompactedObjects CompactedObjects `json:"compactedObjects,omitempty"`
}

func (ccr *CompactedCommandResult) ToNonCompacted() *CommandResult {
	ret := ccr.CommandResult
	if ccr.CompactedObjects != nil {
		ret.Objects = ccr.CompactedObjects
	}
	return &ret
}
