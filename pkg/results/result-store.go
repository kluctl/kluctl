package results

import (
	"github.com/kluctl/kluctl/v2/pkg/types/result"
)

type ListProjectsOptions struct {
	ProjectFilter *result.ProjectKey `json:"projectFilter,omitempty"`
}

type ListTargetsOptions struct {
	ProjectFilter *result.ProjectKey `json:"projectFilter,omitempty"`
}

type ListCommandResultSummariesOptions struct {
	ProjectFilter *result.ProjectKey `json:"projectFilter,omitempty"`
}

type GetCommandResultOptions struct {
	Id      string `json:"id"`
	Reduced bool   `json:"reduced,omitempty"`
}

type ResultStore interface {
	WriteCommandResult(cr *result.CommandResult) error

	ListCommandResultSummaries(options ListCommandResultSummariesOptions) ([]result.CommandResultSummary, error)
	WatchCommandResultSummaries(options ListCommandResultSummariesOptions, update func(summary *result.CommandResultSummary), delete func(id string)) (func(), error)
	HasCommandResult(id string) (bool, error)
	GetCommandResultSummary(id string) (*result.CommandResultSummary, error)
	GetCommandResult(options GetCommandResultOptions) (*result.CommandResult, error)
}

func FilterSummary(x *result.CommandResultSummary, filter *result.ProjectKey) bool {
	if filter == nil {
		return true
	}
	if x.ProjectKey.NormalizedGitUrl != filter.NormalizedGitUrl {
		return false
	}
	if x.ProjectKey.SubDir != filter.SubDir {
		return false
	}
	return true
}
