package main

import (
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/kluctl_project"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func runCmdListTargets(cmd *cobra.Command, args_ []string) error {
	return withKluctlProjectFromArgs(func(p *kluctl_project.KluctlProjectContext) error {
		var result []*types.Target
		for _, t := range p.DynamicTargets {
			result = append(result, t.Target)
		}
		return outputYamlResult(args.Output, result, false)
	})
}

func init() {
	var cmd = &cobra.Command{
		Use:   "list-targets",
		Short: "Outputs a yaml list with all target, including dynamic targets",
		Long: "Outputs a yaml list with all target, including dynamic targets",
		RunE: runCmdListTargets,
	}
	args.AddProjectArgs(cmd, true, true, false)
	args.AddMiscArguments(cmd, args.EnabledMiscArguments{
		Output: true,
	})
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cmd)
}
