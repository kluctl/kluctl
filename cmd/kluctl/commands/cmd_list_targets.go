package commands

import (
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/types"
)

type listTargetsCmd struct {
	args.ProjectFlags
	args.OutputFlags
}

func (cmd *listTargetsCmd) Help() string {
	return `Outputs a yaml list with all target, including dynamic targets`
}

func (cmd *listTargetsCmd) Run() error {
	return withKluctlProjectFromArgs(cmd.ProjectFlags, func(p *kluctl_project.KluctlProjectContext) error {
		var result []*types.Target
		for _, t := range p.DynamicTargets {
			result = append(result, t.Target)
		}
		return outputYamlResult(cmd.Output, result, false)
	})
}
