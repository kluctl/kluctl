package commands

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	git2 "github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/helm"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
		return fmt.Errorf("helm-update can only be used on the root of a Kluctl project that must have a .kluctl.yaml file")
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
		for pth, s := range gitStatus {
			if strings.HasPrefix(pth, ".helm-charts/") {
				status.Trace(ctx, "gitStatus=%s", gitStatus.String())
				return fmt.Errorf("--commit can only be used when .helm-chart directory is clean")
			}
			if (s.Staging != git.Untracked && s.Staging != git.Unmodified) || (s.Worktree != git.Untracked && s.Worktree != git.Unmodified) {
				status.Trace(ctx, "gitStatus=%s", gitStatus.String())
				return fmt.Errorf("--commit can only be used when the git worktree is unmodified")
			}
		}
	}

	baseChartsDir := filepath.Join(projectDir, ".helm-charts")

	g := utils.NewGoHelper(ctx, 8)

	releases, charts, err := loadHelmReleases(projectDir, baseChartsDir, &cmd.HelmCredentials)
	if err != nil {
		return err
	}

	if cmd.Commit {
		actions, err := doHelmPull(ctx, projectDir, &cmd.HelmCredentials, true, false)
		if err != nil {
			return err
		}
		if actions != 0 {
			return fmt.Errorf(".helm-charts is not up-to-date. Please run helm-pull before")
		}
	}

	for _, chart := range charts {
		chart := chart
		g.RunE(func() error {
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
	g.Wait()
	if g.ErrorOrNil() != nil {
		return g.ErrorOrNil()
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
			g.RunE(func() error {
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
	g.Wait()
	if g.ErrorOrNil() != nil {
		return g.ErrorOrNil()
	}

	upgrades := map[helmUpgradeKey][]*helm.Release{}

	for _, hr := range releases {
		cd, err := hr.Chart.BuildPulledChartDir(baseChartsDir, "")
		if err != nil {
			return err
		}

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

		status.Info(ctx, "%s: Chart %s has new version %s available", relDir, hr.Chart.GetChartName(), latestVersion)

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

		k := helmUpgradeKey{
			chartDir:   cd,
			oldVersion: oldVersion,
			newVersion: latestVersion,
		}
		upgrades[k] = append(upgrades[k], hr)
	}

	for k, hrs := range upgrades {
		err = cmd.pullAndCommit(ctx, projectDir, baseChartsDir, gitRootPath, hrs, k.oldVersion)
		if err != nil {
			return err
		}
	}

	return nil
}

func (cmd *helmUpdateCmd) collectFiles(root string, dir string, m map[string]os.FileInfo) error {
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
		m[relToGit], _ = d.Info()
		return nil
	})
	if os.IsNotExist(err) {
		err = nil
	}
	return err
}

func (cmd *helmUpdateCmd) pullAndCommit(ctx context.Context, projectDir string, baseChartsDir string, gitRootPath string, hrs []*helm.Release, oldVersion string) error {
	chart := hrs[0].Chart
	newVersion := hrs[0].Config.ChartVersion

	s := status.Start(ctx, "Upgrading Chart %s from version %s to %s", chart.GetChartName(), oldVersion, newVersion)
	defer s.Failed()

	doError := func(err error) error {
		s.FailedWithMessage("%s", err.Error())
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

	if cmd.Commit {
		for _, hr := range hrs {
			// add helm-chart.yaml
			relToGit, err := filepath.Rel(gitRootPath, hr.ConfigFile)
			if err != nil {
				return doError(err)
			}
			_, err = wt.Add(relToGit)
			if err != nil {
				return doError(err)
			}
		}
	}

	// we need to list all files contained inside the charts dir BEFORE doing the pull, so that we later
	// know what got deleted
	files := map[string]os.FileInfo{}
	if cmd.Commit {
		err = cmd.collectFiles(gitRootPath, baseChartsDir, files)
		if err != nil {
			return doError(err)
		}
	}

	_, err = doHelmPull(ctx, projectDir, &cmd.HelmCredentials, false, false)
	if err != nil {
		return doError(err)
	}

	if cmd.Commit {
		files2 := map[string]os.FileInfo{}
		err = cmd.collectFiles(gitRootPath, baseChartsDir, files2)
		if err != nil {
			return doError(err)
		}

		for pth, st1 := range files {
			st2, ok := files2[pth]
			if !ok || st1.Mode() != st2.Mode() || st1.ModTime() != st2.ModTime() || st1.Size() != st2.Size() {
				// removed or modified
				if ok {
					if !st2.IsDir() {
						_, err = wt.Add(pth)
					}
				} else {
					if !st1.IsDir() {
						_, err = wt.Remove(pth)
					}
				}
				if err != nil && err != index.ErrEntryNotFound {
					return doError(err)
				}
			}
		}
		for pth, st1 := range files2 {
			if _, ok := files[pth]; !ok {
				if !st1.IsDir() {
					// added
					_, err = wt.Add(pth)
					if err != nil && err != index.ErrEntryNotFound {
						return doError(err)
					}
				}
			}
		}

		commitMsg := fmt.Sprintf("Updated helm chart %s from version %s to version %s", chart.GetChartName(), oldVersion, newVersion)
		_, err = wt.Commit(commitMsg, &git.CommitOptions{})
		if err != nil {
			return doError(fmt.Errorf("failed to commit: %w", err))
		}

		s.UpdateAndInfoFallback("Committed helm chart %s with version %s", chart.GetChartName(), newVersion)
	}
	s.Success()

	return nil
}

type helmUpgradeKey struct {
	chartDir   string
	oldVersion string
	newVersion string
}
