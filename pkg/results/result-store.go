package results

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
)

type ListProjectsOptions struct {
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
	WriteCommandResult(ctx context.Context, cr *result.CommandResult) error

	ListProjects(ctx context.Context, options ListProjectsOptions) ([]result.ProjectSummary, error)
	ListCommandResultSummaries(ctx context.Context, options ListCommandResultSummariesOptions) ([]result.CommandResultSummary, error)
	GetCommandResult(ctx context.Context, options GetCommandResultOptions) (*result.CommandResult, error)
}
