package commands

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	git2 "github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"io/fs"
	"os"
	"path/filepath"
)

type helmPullCmd struct {
	args.HelmCredentials
}

func (cmd *helmPullCmd) Help() string {
	return `The Helm charts are stored under the sub-directory 'charts/<chart-name>' next to the
'helm-chart.yaml'. These Helm charts are meant to be added to version control so that
pulling is only needed when really required (e.g. when the chart version changes).`
}

func (cmd *helmPullCmd) Run() error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	gitRootPath, err := git2.DetectGitRepositoryRoot(cwd)
	if err != nil {
		return err
	}

	err = filepath.WalkDir(cwd, func(p string, d fs.DirEntry, err error) error {
		fname := filepath.Base(p)
		if fname == "helm-chart.yml" || fname == "helm-chart.yaml" {
			statusPrefix, err := filepath.Rel(gitRootPath, filepath.Dir(p))
			if err != nil {
				return err
			}

			s := status.Start(cliCtx, "%s: Pulling Chart %s", statusPrefix)
			chart, err := deployment.NewHelmChart(p)
			if err != nil {
				s.FailedWithMessage("%s: %s", statusPrefix, err.Error())
				return err
			}

			chartName, err := chart.GetChartName()
			if err != nil {
				s.FailedWithMessage("%s: %s", statusPrefix, err.Error())
				return err
			}

			s.Update("%s: Pulling Chart %s with version %s", statusPrefix, chartName, *chart.Config.ChartVersion)

			creds := cmd.HelmCredentials.FindCredentials(*chart.Config.Repo, chart.Config.CredentialsId)
			if chart.Config.CredentialsId != nil && creds == nil {
				err := fmt.Errorf("%s: no credentials provided", statusPrefix)
				s.FailedWithMessage(err.Error())
				return err
			}
			chart.SetCredentials(creds)

			err = chart.Pull(cliCtx)
			if err != nil {
				s.FailedWithMessage("%s: %s", statusPrefix, err.Error())
				return err
			}
			s.Success()
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("command failed")
	}

	return err
}
