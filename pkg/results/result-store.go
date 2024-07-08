package results

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	gittypes "github.com/kluctl/kluctl/v2/lib/git/types"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
)

type ListResultSummariesOptions struct {
	ProjectFilter *gittypes.ProjectKey `json:"projectFilter,omitempty"`
}

type GetCommandResultOptions struct {
	Id      string `json:"id"`
	Reduced bool   `json:"reduced,omitempty"`
}

type GetValidateResultOptions struct {
	Id string `json:"id"`
}

type WatchCommandResultSummaryEvent struct {
	Summary *result.CommandResultSummary `json:"summary"`
	Delete  bool                         `json:"delete"`
}

type WatchValidateResultSummaryEvent struct {
	Summary *result.ValidateResultSummary `json:"summary"`
	Delete  bool                          `json:"delete"`
}

type WatchKluctlDeploymentEvent struct {
	ClusterId  string                     `json:"clusterId"`
	Deployment *kluctlv1.KluctlDeployment `json:"deployment"`
	Delete     bool                       `json:"delete"`
}

type ResultStore interface {
	WriteCommandResult(cr *result.CommandResult) error
	WriteValidateResult(vr *result.ValidateResult) error
	DeleteCommandResult(rsId string) error

	ListCommandResultSummaries(options ListResultSummariesOptions) ([]result.CommandResultSummary, error)
	WatchCommandResultSummaries(options ListResultSummariesOptions) (<-chan WatchCommandResultSummaryEvent, context.CancelFunc, error)
	GetCommandResult(options GetCommandResultOptions) (*result.CommandResult, error)

	ListValidateResultSummaries(options ListResultSummariesOptions) ([]result.ValidateResultSummary, error)
	WatchValidateResultSummaries(options ListResultSummariesOptions) (<-chan WatchValidateResultSummaryEvent, context.CancelFunc, error)
	GetValidateResult(options GetValidateResultOptions) (*result.ValidateResult, error)

	ListKluctlDeployments() ([]WatchKluctlDeploymentEvent, error)
	WatchKluctlDeployments() (<-chan WatchKluctlDeploymentEvent, context.CancelFunc, error)
	GetKluctlDeployment(clusterId string, name string, namespace string) (*kluctlv1.KluctlDeployment, error)
}

func FilterProject(x gittypes.ProjectKey, filter *gittypes.ProjectKey) bool {
	if filter != nil {
		if filter.RepoKey.String() != "" && x.RepoKey != filter.RepoKey {
			return false
		}
		if filter.SubDir != "" && x.SubDir != filter.SubDir {
			return false
		}
	}
	return true
}

func lessCommandSummary(a *result.CommandResultSummary, b *result.CommandResultSummary) bool {
	if a.Command.StartTime != b.Command.StartTime {
		return a.Command.StartTime.After(b.Command.StartTime.Time)
	}
	if a.Command.EndTime != b.Command.EndTime {
		return a.Command.EndTime.After(b.Command.EndTime.Time)
	}
	if a.Command.Command != b.Command.Command {
		return a.Command.Command < b.Command.Command
	}
	return a.Id < b.Id
}

func lessValidateSummary(a *result.ValidateResultSummary, b *result.ValidateResultSummary) bool {
	if a.StartTime != b.StartTime {
		return a.StartTime.After(b.StartTime.Time)
	}
	if a.EndTime != b.EndTime {
		return a.EndTime.After(b.EndTime.Time)
	}
	return a.Id < b.Id
}
