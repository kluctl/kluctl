package commands

import (
	"context"
	"strings"

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
	return withKluctlProjectFromArgs(ctx, nil, cmd.ProjectFlags, nil, nil, nil, nil, false, true, false, func(ctx context.Context, p *kluctl_project.LoadedKluctlProject) error {
		var result []*types.Target

		for _, t := range p.Targets {
			result = append(result, t)
		}

		if cmd.OnlyNames {
			var targetNames []string
			for _, target := range result {
				targetNames = append(targetNames, target.Name)
			}
			targetNamesStr := strings.Join(targetNames, "\n")
			if targetNamesStr != "" {
				targetNamesStr += "\n"
			}
			return outputResult2(ctx, cmd.Output, targetNamesStr)
		}
		return outputYamlResult(ctx, cmd.Output, result, false)
	})
}
