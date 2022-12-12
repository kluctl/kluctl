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

	releases, charts, err := loadHelmReleases(projectDir, baseChartsDir, &cmd.HelmCredentials)
	if err != nil {
		return err
	}

	for _, hr := range releases {
		if utils.Exists(hr.GetDeprecatedChartDir()) {
			rel, err := filepath.Rel(projectDir, hr.GetDeprecatedChartDir())
			if err != nil {
				return err
			}
			status.Info(ctx, "Removing deprecated charts dir %s", rel)
			err = os.RemoveAll(hr.GetDeprecatedChartDir())
			if err != nil {
				return err
			}
		}
	}

	var errs *multierror.Error
	var wg sync.WaitGroup
	var mutex sync.Mutex
	sem := semaphore.NewWeighted(8)

	for _, chart := range charts {
		chart := chart
		statusPrefix := chart.GetChartName()

		versionsToPull := map[string]bool{}
		for _, hr := range releases {
			if hr.Chart == chart {
				versionsToPull[hr.Config.ChartVersion] = true
			}
		}

		cleanupDir, err := chart.BuildPulledChartDir(baseChartsDir, "")
		if err != nil {
			return err
		}
		des, err := os.ReadDir(cleanupDir)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		for _, de := range des {
			if !de.IsDir() {
				continue
			}
			if _, ok := versionsToPull[de.Name()]; !ok {
				status.Info(ctx, "Removing unused Chart with version %s", de.Name())
				err = os.RemoveAll(filepath.Join(cleanupDir, de.Name()))
				if err != nil {
					return err
				}
			}
		}

		for version, _ := range versionsToPull {
			version := version
			utils.GoLimitedMultiError(ctx, sem, &errs, &mutex, &wg, func() error {
				s := status.Start(ctx, "%s: Pulling Chart with version %s", statusPrefix, version)
				defer s.Failed()

				_, err := chart.PullInProject(ctx, baseChartsDir, version)
				if err != nil {
					s.FailedWithMessage("%s: %s", statusPrefix, err.Error())
					return err
				}

				s.Success()
				return nil
			})
		}
	}
	wg.Wait()
	if err != nil {
		errs = multierror.Append(errs, err)
	}

	if errs.ErrorOrNil() != nil {
		return fmt.Errorf("command failed")
	}

	return nil
}

func loadHelmReleases(projectDir string, baseChartsDir string, credentialsProvider helm.HelmCredentialsProvider) ([]*helm.Release, []*helm.Chart, error) {
	var releases []*helm.Release
	chartsMap := make(map[string]*helm.Chart)
	err := filepath.WalkDir(projectDir, func(p string, d fs.DirEntry, err error) error {
		fname := filepath.Base(p)
		if fname != "helm-chart.yml" && fname != "helm-chart.yaml" {
			return nil
		}

		hr, err := helm.NewRelease(p, baseChartsDir, credentialsProvider)
		if err != nil {
			return err
		}

		releases = append(releases, hr)
		chart := hr.Chart
		key := fmt.Sprintf("%s / %s", chart.GetRepo(), chart.GetChartName())
		if x, ok := chartsMap[key]; !ok {
			chartsMap[key] = chart
		} else {
			hr.Chart = x
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	charts := make([]*helm.Chart, 0, len(chartsMap))
	for _, chart := range chartsMap {
		charts = append(charts, chart)
	}
	return releases, charts, nil
}
