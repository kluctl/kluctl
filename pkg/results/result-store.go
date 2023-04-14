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

	ListCommandResultSummaries(ctx context.Context, options ListCommandResultSummariesOptions) ([]result.CommandResultSummary, error)
	GetCommandResult(ctx context.Context, options GetCommandResultOptions) (*result.CommandResult, error)
}

func ListProjects(ctx context.Context, rs ResultStore, options ListProjectsOptions) ([]result.ProjectSummary, error) {
	summaries, err := rs.ListCommandResultSummaries(ctx, ListCommandResultSummariesOptions{
		ProjectFilter: options.ProjectFilter,
	})
	if err != nil {
		return nil, err
	}

	m := map[result.ProjectKey]result.ProjectSummary{}
	for _, s := range summaries {
		if _, ok := m[s.Project]; !ok {
			m[s.Project] = result.ProjectSummary{Project: s.Project}
		}
	}

	ret := make([]result.ProjectSummary, 0, len(m))
	for _, p := range m {
		for _, rs := range summaries {
			switch rs.Command.Command {
			case "deploy":
				if p.LastDeployCommand == nil {
					rs := rs
					p.LastDeployCommand = &rs
				}
			case "delete":
				if p.LastDeleteCommand == nil {
					rs := rs
					p.LastDeleteCommand = &rs
				}
			case "prune":
				if p.LastPruneCommand == nil {
					rs := rs
					p.LastPruneCommand = &rs
				}
			}
		}
		ret = append(ret, p)
	}

	return ret, nil
}
