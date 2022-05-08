package commands

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"io/fs"
	"path/filepath"
)

type helmPullCmd struct {
	LocalDeployment string `group:"project" help:"Local deployment directory. Defaults to current directory"`
}

func (cmd *helmPullCmd) Help() string {
	return `The Helm charts are stored under the sub-directory 'charts/<chart-name>' next to the
'helm-chart.yml'. These Helm charts are meant to be added to version control so that
pulling is only needed when really required (e.g. when the chart version changes).`
}

func (cmd *helmPullCmd) Run(ctx context.Context) error {
	rootPath := "."
	if cmd.LocalDeployment != "" {
		rootPath = cmd.LocalDeployment
	}
	err := filepath.WalkDir(rootPath, func(p string, d fs.DirEntry, err error) error {
		fname := filepath.Base(p)
		if fname == "helm-chart.yml" || fname == "helm-chart.yaml" {
			s := status.Start(ctx, "Pulling for %s", p)
			chart, err := deployment.NewHelmChart(p)
			if err != nil {
				s.FailedWithMessage(err.Error())
				return err
			}
			err = chart.Pull(ctx)
			if err != nil {
				s.FailedWithMessage(err.Error())
				return err
			}
			s.Success()
		}
		return nil
	})
	return err
}
