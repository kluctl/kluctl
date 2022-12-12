package commands

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/hashicorp/go-multierror"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	git2 "github.com/kluctl/kluctl/v2/pkg/git"
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

type helmUpdateCmd struct {
	args.ProjectDir
	args.HelmCredentials

	Upgrade bool `group:"misc" help:"Write new versions into helm-chart.yaml and perform helm-pull afterwards"`
	Commit  bool `group:"misc" help:"Create a git commit for every updated chart"`

	Interactive bool `group:"misc" short:"i" help:"Ask for every Helm Chart if it should be upgraded."`
}

func (cmd *helmUpdateCmd) Help() string {
	return `Optionally performs the actual upgrade and/or add a commit to version control.`
}

func (cmd *helmUpdateCmd) Run(ctx context.Context) error {
	projectDir, err := cmd.ProjectDir.GetProjectDir()
	if err != nil {
		return err
	}

	if !yaml.Exists(filepath.Join(projectDir, ".kluctl.yaml")) {
		return fmt.Errorf("helm-pull can only be used on the root of a Kluctl project that must have a .kluctl.yaml file")
	}

	gitRootPath, err := git2.DetectGitRepositoryRoot(projectDir)
	if err != nil {
		return err
	}

	if cmd.Commit {
		g, err := git.PlainOpen(gitRootPath)
		if err != nil {
			return err
		}
		wt, err := g.Worktree()
		if err != nil {
			return err
		}
		gitStatus, err := wt.Status()
		if err != nil {
			return err
		}
		for _, s := range gitStatus {
			if (s.Staging != git.Untracked && s.Staging != git.Unmodified) || (s.Worktree != git.Untracked && s.Worktree != git.Unmodified) {
				return fmt.Errorf("--commit can only be used when the git worktree is unmodified")
			}
		}
	}

	baseChartsDir := filepath.Join(projectDir, ".helm-charts")

	var errs *multierror.Error
	var wg sync.WaitGroup
	var mutex sync.Mutex
	sem := semaphore.NewWeighted(8)

	releases, charts, err := loadHelmReleases(projectDir, baseChartsDir, &cmd.HelmCredentials)
	if err != nil {
		return err
	}

	for _, hr := range releases {
		if utils.Exists(hr.GetDeprecatedChartDir()) {
			relDir, err := filepath.Rel(projectDir, filepath.Dir(hr.ConfigFile))
			if err != nil {
				return err
			}
			status.Error(ctx, "%s: Project is using a pre-pulled Helm Chart that is next to the helm-chart.yaml, which is deprecated. "+
				"Updating is only possible after removing these. Use 'kluctl helm-pull' to remove all deprecated chart folders.", relDir)
			return fmt.Errorf("detected deprecated chart folder")
		}
	}

	for _, chart := range charts {
		chart := chart
		utils.GoLimitedMultiError(ctx, sem, &errs, &mutex, &wg, func() error {
			s := status.Start(ctx, "%s: Querying versions", chart.GetChartName())
			defer s.Failed()
			err := chart.QueryVersions(ctx)
			if err != nil {
				s.FailedWithMessage("%s: %s", chart.GetChartName(), err.Error())
				return err
			}
			s.Success()
			return nil
		})
	}
	wg.Wait()
	if errs.ErrorOrNil() != nil {
		return errs
	}

	for _, chart := range charts {
		chart := chart

		versionsToPull := map[string]bool{}
		for _, hr := range releases {
			if hr.Chart != chart {
				continue
			}
			versionsToPull[hr.Config.ChartVersion] = true
		}

		for version, _ := range versionsToPull {
			version := version
			utils.GoLimitedMultiError(ctx, sem, &errs, &mutex, &wg, func() error {
				s := status.Start(ctx, "%s: Downloading Chart with version %s into cache", chart.GetChartName(), version)
				defer s.Failed()
				_, err := chart.PullCached(ctx, version)
				if err != nil {
					s.FailedWithMessage("%s: %s", chart.GetChartName(), err.Error())
					return err
				}
				s.Success()
				return nil
			})
		}
	}
	wg.Wait()
	if errs.ErrorOrNil() != nil {
		return errs
	}

	versionUseCounts := map[string]map[string]int{}
	for _, hr := range releases {
		key := fmt.Sprintf("%s / %s", hr.Chart.GetRepo(), hr.Chart.GetChartName())
		if _, ok := versionUseCounts[key]; !ok {
			versionUseCounts[key] = map[string]int{}
		}
		versionUseCounts[key][hr.Config.ChartVersion]++
	}

	for _, hr := range releases {
		relDir, err := filepath.Rel(projectDir, filepath.Dir(hr.ConfigFile))
		if err != nil {
			return err
		}

		latestVersion, err := hr.Chart.GetLatestVersion(hr.Config.UpdateConstraints)
		if err != nil {
			return err
		}
		if hr.Config.ChartVersion == latestVersion {
			continue
		}

		if hr.Config.SkipUpdate {
			status.Info(ctx, "%s: Skipped update to version %s", relDir, latestVersion)
			continue
		}

		status.Info(ctx, "%s: Chart has new version %s available", relDir, latestVersion)

		if !cmd.Upgrade {
			continue
		}

		if cmd.Interactive {
			if !status.AskForConfirmation(ctx, fmt.Sprintf("%s: Do you want to upgrade Chart %s to version %s?",
				relDir, hr.Chart.GetChartName(), latestVersion)) {
				continue
			}
		}

		oldVersion := hr.Config.ChartVersion
		hr.Config.ChartVersion = latestVersion
		err = hr.Save()
		if err != nil {
			return err
		}

		status.Info(ctx, "%s: Updated Chart version to %s", relDir, latestVersion)

		if !cmd.Commit {
			continue
		}

		key := fmt.Sprintf("%s / %s", hr.Chart.GetRepo(), hr.Chart.GetChartName())
		uv := versionUseCounts[key]
		uv[oldVersion]--
		uv[latestVersion]++

		pullChart := false
		deleteChart := false
		if uv[latestVersion] == 1 {
			pullChart = true
		}
		if uv[oldVersion] == 0 {
			deleteChart = true
		}

		err = cmd.pullAndCommitCharts(ctx, projectDir, baseChartsDir, gitRootPath, hr, oldVersion, latestVersion, pullChart, deleteChart)
		if err != nil {
			return err
		}
	}

	return errs.ErrorOrNil()
}

func (cmd *helmUpdateCmd) collectFiles(root string, dir string, m map[string]bool) error {
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if d == nil || d.IsDir() {
			return nil
		}
		relToGit, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		if _, ok := m[relToGit]; ok {
			return nil
		}
		m[relToGit] = true
		return nil
	})
	if os.IsNotExist(err) {
		err = nil
	}
	return err
}

func (cmd *helmUpdateCmd) pullAndCommitCharts(ctx context.Context, projectDir string, baseChartsDir string, gitRootPath string, hr *helm.Release, oldVersion string, newVersion string, pullChart bool, deleteChart bool) error {
	relDir, err := filepath.Rel(projectDir, filepath.Dir(hr.ConfigFile))
	if err != nil {
		return err
	}

	s := status.Start(ctx, "%s: Committing Chart with version %s", relDir, newVersion)
	defer s.Failed()

	doError := func(err error) error {
		s.FailedWithMessage("%s: %s", relDir, err.Error())
		return err
	}

	r, err := git.PlainOpen(gitRootPath)
	if err != nil {
		return doError(err)
	}
	wt, err := r.Worktree()
	if err != nil {
		return doError(err)
	}

	// add helm-chart.yaml
	relToGit, err := filepath.Rel(gitRootPath, hr.ConfigFile)
	if err != nil {
		return doError(err)
	}
	_, err = wt.Add(relToGit)
	if err != nil {
		return doError(err)
	}

	if deleteChart {
		chartDir, err := hr.Chart.BuildPulledChartDir(baseChartsDir, oldVersion)
		if err != nil {
			return doError(err)
		}
		relChartDir, err := filepath.Rel(gitRootPath, chartDir)
		if err != nil {
			return doError(err)
		}
		_, err = wt.Remove(relChartDir)
		if err != nil && err != index.ErrEntryNotFound {
			return doError(err)
		}
		err = os.RemoveAll(chartDir)
		if err != nil {
			return doError(err)
		}
	}

	if pullChart {
		chartDir, err := hr.Chart.BuildPulledChartDir(baseChartsDir, newVersion)
		if err != nil {
			return doError(err)
		}
		relChartDir, err := filepath.Rel(gitRootPath, chartDir)
		if err != nil {
			return doError(err)
		}

		// we need to list all files contained inside the charts dir BEFORE doing the pull, so that we later
		// know what got deleted
		files := map[string]bool{}
		err = cmd.collectFiles(gitRootPath, chartDir, files)
		if err != nil {
			return doError(err)
		}

		_, err = hr.Chart.PullInProject(ctx, baseChartsDir, newVersion)
		if err != nil {
			return doError(err)
		}

		_, err = wt.Add(relChartDir)
		if err != nil {
			return doError(err)
		}

		// figure out what got deleted
		for p := range files {
			_, err := os.Lstat(filepath.Join(gitRootPath, p))
			if err != nil {
				if os.IsNotExist(err) {
					_, err = wt.Remove(p)
					if err != nil {
						return doError(err)
					}
				} else {
					return doError(err)
				}
			}
		}
	}

	commitMsg := fmt.Sprintf("Updated helm chart %s to version %s", relToGit, newVersion)
	_, err = wt.Commit(commitMsg, &git.CommitOptions{})
	if err != nil {
		return doError(fmt.Errorf("failed to commit: %w", err))
	}

	s.Update("%s: Committed helm chart with version %s", relToGit, newVersion)
	s.Success()

	return nil
}
