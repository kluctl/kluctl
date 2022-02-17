package commands

import (
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/deployment"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/fs"
	"path"
	"path/filepath"
)

func runCmdHelmPull(cmd *cobra.Command, args_ []string) error {
	rootPath := "."
	if args.LocalDeployment != "" {
		rootPath = args.LocalDeployment
	}
	err := filepath.WalkDir(rootPath, func(p string, d fs.DirEntry, err error) error {
		fname := path.Base(p)
		if fname == "helm-chart.yml" {
			log.Infof("Pulling for %s", p)
			chart, err := deployment.NewHelmChart(p)
			if err != nil {
				return err
			}
			err = chart.Pull()
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func init() {
	var cmd = &cobra.Command{
		Use:   "helm-pull",
		Short: "Recursively searches for `helm-chart.yml` files and pulls the specified Helm charts",
		Long: "Recursively searches for `helm-chart.yml` files and pulls the specified Helm charts.\n\n" +
			"The Helm charts are stored under the sub-directory `charts/<chart-name>` next to the " +
			"`helm-chart.yml`. These Helm charts are meant to be added to version control so that " +
			"pulling is only needed when really required (e.g. when the chart version changes).",
		RunE: runCmdHelmPull,
	}
	cmd.Flags().StringVar(&args.LocalDeployment, "local-deployment", "", "Local deployment directory. Defaults to current directory")
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cmd)
}
