package commands

import (
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var simple = false

func runCmdListImages(cmd *cobra.Command, args_ []string) error {
	return withProjectCommandContext(func(ctx *commandCtx) error {
		result := ctx.images.SeenImages(simple)
		return outputYamlResult(args.Output, result, false)
	})
}

func init() {
	var cmd = &cobra.Command{
		Use:   "list-images",
		Short: "Renders the target and outputs all images used via `images.get_image(...)",
		Long: "Renders the target and outputs all images used via `images.get_image(...)`\n\n"+
		"The result is a compatible with yaml files expected by --fixed-images-file.\n\n"+
		"If fixed images (`-f/--fixed-image`) are provided, these are also taken into account, "+
		"as described in for the deploy command.",
		RunE: runCmdListImages,
	}
	args.AddProjectArgs(cmd, true, true, true)
	args.AddImageArgs(cmd)
	args.AddInclusionArgs(cmd)
	args.AddMiscArguments(cmd, args.EnabledMiscArguments{
		Output: true,
	})
	cmd.Flags().BoolVar(&simple, "simple", false, "Output a simplified version of the images list")
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cmd)
}
