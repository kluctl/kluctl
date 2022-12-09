package commands

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	git2 "github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"golang.org/x/sync/semaphore"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

type helmPullCmd struct {
	args.HelmCredentials
}

func (cmd *helmPullCmd) Help() string {
	return `The Helm charts are stored under the sub-directory 'charts/<chart-name>' next to the
'helm-chart.yaml'. These Helm charts are meant to be added to version control so that
pulling is only needed when really required (e.g. when the chart version changes).`
}

func (cmd *helmPullCmd) Run(ctx context.Context) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	gitRootPath, err := git2.DetectGitRepositoryRoot(cwd)
	if err != nil {
		return err
	}

	var errs *multierror.Error
	var wg sync.WaitGroup
	var mutex sync.Mutex
	sem := semaphore.NewWeighted(8)

	err = filepath.WalkDir(cwd, func(p string, d fs.DirEntry, err error) error {
		fname := filepath.Base(p)
		if fname != "helm-chart.yml" && fname != "helm-chart.yaml" {
			return nil
		}

		statusPrefix, err := filepath.Rel(gitRootPath, filepath.Dir(p))
		if err != nil {
			return err
		}

		utils.GoLimitedMultiError(ctx, sem, &errs, &mutex, &wg, func() error {
			s := status.Start(ctx, "%s: Pulling Chart", statusPrefix)
			defer s.Failed()
			err := doPull(ctx, statusPrefix, p, cmd.HelmCredentials, s)
			if err != nil {
				return err
			}
			s.Success()
			return nil
		})

		return nil
	})
	wg.Wait()
	if err != nil {
		errs = multierror.Append(errs, err)
	}

	if errs.ErrorOrNil() != nil {
		return fmt.Errorf("command failed")
	}

	return nil
}

func doPull(ctx context.Context, statusPrefix string, p string, helmCredentials args.HelmCredentials, s *status.StatusContext) error {
	doError := func(err error) error {
		s.FailedWithMessage("%s: %s", statusPrefix, err.Error())
		return err
	}

	chart, err := deployment.NewHelmChart(p)
	if err != nil {
		return doError(err)
	}

	chart.SetCredentials(&helmCredentials)

	s.UpdateAndInfoFallback("%s: Pulling Chart %s with version %s", statusPrefix, chart.GetChartName(), *chart.Config.ChartVersion)

	err = chart.Pull(ctx)
	if err != nil {
		return doError(err)
	}
	return nil
}
