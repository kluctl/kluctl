package commands

import (
	"context"

	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/types"
)

type listTargetsCmd struct {
	args.ProjectFlags
	args.OutputFlags

	OnlyNames bool `group:"list-targets" help:"If provided --only-names will only output "`
}

func (cmd *listTargetsCmd) Help() string {
	return `Outputs a yaml list with all targets`
}

func (cmd *listTargetsCmd) Run(ctx context.Context) error {
	return withKluctlProjectFromArgs(ctx, nil, cmd.ProjectFlags, nil, nil, nil, false, true, false, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
		var result []*types.Target

		for _, t := range p.Targets {
			result = append(result, t)
		}

		if cmd.OnlyNames {
			var targetNames []string
			for _, target := range result {
				targetNames = append(targetNames, target.Name)
			}
			return outputYamlResult(ctx, cmd.Output, targetNames, false)
		}
		return outputYamlResult(ctx, cmd.Output, result, false)
	})
}
