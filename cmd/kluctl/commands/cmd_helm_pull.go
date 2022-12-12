package commands

import (
	"context"
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/helm"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"golang.org/x/sync/semaphore"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

type helmPullCmd struct {
	args.ProjectDir
	args.HelmCredentials
}

func (cmd *helmPullCmd) Help() string {
	return `The Helm charts are stored under the sub-directory 'charts/<chart-name>' next to the
'helm-chart.yaml'. These Helm charts are meant to be added to version control so that
pulling is only needed when really required (e.g. when the chart version changes).`
}

func (cmd *helmPullCmd) Run(ctx context.Context) error {
	projectDir, err := cmd.ProjectDir.GetProjectDir()
	if err != nil {
		return err
	}

	if !yaml.Exists(filepath.Join(projectDir, ".kluctl.yaml")) {
		return fmt.Errorf("helm-pull can only be used on the root of a Kluctl project that must have a .kluctl.yaml file")
	}

	baseChartsDir := filepath.Join(projectDir, ".helm-charts")

	chartsToPull := map[string]*helm.HelmChart{}
	cleanupDirs := map[string][]string{}

	err = filepath.WalkDir(projectDir, func(p string, d fs.DirEntry, err error) error {
		fname := filepath.Base(p)
		if fname != "helm-chart.yml" && fname != "helm-chart.yaml" {
			return nil
		}

		chart, err := helm.NewHelmChart(baseChartsDir, p)
		if err != nil {
			return err
		}

		chart.SetCredentials(&cmd.HelmCredentials)

		cleanupDir := chart.GetChartDir(false)
		cleanupDirs[cleanupDir] = append(cleanupDirs[cleanupDir], chart.Config.ChartVersion)

		if _, ok := chartsToPull[chart.GetChartDir(true)]; ok {
			return nil
		}

		chartsToPull[chart.GetChartDir(true)] = chart
		return nil
	})

	var errs *multierror.Error
	var wg sync.WaitGroup
	var mutex sync.Mutex
	sem := semaphore.NewWeighted(8)

	for _, chart := range chartsToPull {
		chart := chart
		statusPrefix := chart.GetChartName()

		utils.GoLimitedMultiError(ctx, sem, &errs, &mutex, &wg, func() error {
			s := status.Start(ctx, "%s: Pulling Chart with version %s", statusPrefix, chart.Config.ChartVersion)
			defer s.Failed()

			err := chart.Pull(ctx)
			if err != nil {
				s.FailedWithMessage("%s: %s", statusPrefix, err.Error())
				return err
			}
			s.Success()
			return nil
		})
	}
	wg.Wait()
	if err != nil {
		errs = multierror.Append(errs, err)
	}

	if errs.ErrorOrNil() != nil {
		return fmt.Errorf("command failed")
	}

	for dir, versions := range cleanupDirs {
		des, err := os.ReadDir(dir)
		if err != nil {
			return err
		}
		for _, de := range des {
			if !de.IsDir() {
				continue
			}
			found := false
			for _, v := range versions {
				if v == de.Name() {
					found = true
					break
				}
			}
			if !found {
				chartName := filepath.Base(dir)
				s := status.Start(ctx, "%s: Removing unused version %s", chartName, de.Name())
				err = os.RemoveAll(filepath.Join(dir, de.Name()))
				if err != nil {
					s.Failed()
					return err
				}
				s.Success()
			}
		}
	}

	return nil
}
