package commands

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/format/index"
	git2 "github.com/kluctl/kluctl/lib/git"
	gitauth "github.com/kluctl/kluctl/lib/git/auth"
	"github.com/kluctl/kluctl/lib/git/messages"
	ssh_pool "github.com/kluctl/kluctl/lib/git/ssh-pool"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/lib/yaml"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/helm"
	helmauth "github.com/kluctl/kluctl/v2/pkg/helm/auth"
	ociauth "github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/prompts"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/utils"
)

type helmUpdateCmd struct {
	args.ProjectDir
	args.HelmCredentials
	args.RegistryCredentials

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

	if !yaml.Exists(filepath.Join(projectDir, ".kluctl.yaml")) && !yaml.Exists(filepath.Join(projectDir, ".kluctl-library.yaml")) {
		return fmt.Errorf("helm-update can only be used on the root of a Kluctl project that must have a .kluctl.yaml or .kluctl-library.yaml file")
	}

	gitRootPath, err := git2.DetectGitRepositoryRoot(projectDir)
	if err != nil {
		return err
	}
	sshPool := &ssh_pool.SshPool{}
	messageCallbacks := &messages.MessageCallbacks{
		WarningFn:            func(s string) { status.Warning(ctx, s) },
		TraceFn:              func(s string) { status.Trace(ctx, s) },
		AskForPasswordFn:     func(s string) (string, error) { return prompts.AskForPassword(ctx, s) },
		AskForConfirmationFn: func(s string) bool { return prompts.AskForConfirmation(ctx, s) },
	}
	gitAuthProvider := gitauth.NewDefaultAuthProviders("KLUCTL_GIT", messageCallbacks)
	ociAuthProvider := ociauth.NewDefaultAuthProviders("KLUCTL_REGISTRY")
	helmAuthProvider := helmauth.NewDefaultAuthProviders("KLUCTL_HELM")
	if x, err := cmd.HelmCredentials.BuildAuthProvider(ctx); err != nil {
		return err
	} else {
		helmAuthProvider.RegisterAuthProvider(x, false)
	}
	if x, err := cmd.RegistryCredentials.BuildAuthProvider(ctx); err != nil {
		return err
	} else {
		ociAuthProvider.RegisterAuthProvider(x, false)
	}
	gitRp := repocache.NewGitRepoCache(ctx, sshPool, gitAuthProvider, nil, time.Second*60)
	defer gitRp.Clear()

	ociRp := repocache.NewOciRepoCache(ctx, ociAuthProvider, nil, time.Second*60)

	defer ociRp.Clear()
	if cmd.Commit {
		gitStatus, err := git2.GetWorktreeStatus(ctx, gitRootPath)
		if err != nil {
			return err
		}
		for pth, s := range gitStatus {
			if strings.HasPrefix(pth, ".helm-charts/") {
				status.Tracef(ctx, "gitStatus=%s", gitStatus.String())
				return fmt.Errorf("--commit can only be used when .helm-chart directory is clean")
			}
			if (s.Staging != git.Untracked && s.Staging != git.Unmodified) || (s.Worktree != git.Untracked && s.Worktree != git.Unmodified) {
				status.Tracef(ctx, "gitStatus=%s", gitStatus.String())
				return fmt.Errorf("--commit can only be used when the git worktree is unmodified")
			}
		}
	}

	baseChartsDir := filepath.Join(projectDir, ".helm-charts")

	g := utils.NewGoHelper(ctx, 8)

	releases, charts, err := loadHelmReleases(ctx, projectDir, baseChartsDir, helmAuthProvider, ociAuthProvider, gitRp, ociRp)
	if err != nil {
		return err
	}

	if cmd.Commit {
		actions, err := doHelmPull(ctx, projectDir, helmAuthProvider, ociAuthProvider, gitRp, ociRp, true, false)
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
			s := status.Startf(ctx, "%s: Querying versions", chart.GetChartName())
			defer s.Failed()
			err := chart.QueryVersions(ctx)
			if err != nil {
				s.FailedWithMessagef("%s: %s", chart.GetChartName(), err.Error())
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
			if hr.Config.IsRegistryChart() {
				versionsToPull[hr.Config.ChartVersion] = true
			}
			if hr.Config.IsGitRepositoryChart() {
				ref, _, err := hr.Config.GetGitRef()
				if err != nil {
					return err
				}
				versionsToPull[ref] = true
			}
		}

		for version, _ := range versionsToPull {
			var out string
			if chart.IsRegistryChart() {
				out = fmt.Sprintf("%s: Downloading Chart with version %s into cache", chart.GetChartName(), version)
			}
			if chart.IsGitRepositoryChart() {
				ref, _, err := chart.GetGitRef()
				if err != nil {
					return err
				}
				out = fmt.Sprintf("%s: Downloading Chart with branch, tag or commit %s into cache", chart.GetChartName(), ref)
			}
			g.RunE(func() error {
				s := status.Startf(ctx, out)
				defer s.Failed()
				_, err := chart.PullCached(ctx, version)
				if err != nil {
					s.FailedWithMessagef("%s: %s", chart.GetChartName(), err.Error())
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
		cd, err := hr.Chart.BuildPulledChartDir(baseChartsDir)
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
		currentVersion, err := hr.Config.GetAbstractVersion()
		if err != nil {
			return err
		}

		if currentVersion == latestVersion {
			continue
		}

		if hr.Config.SkipUpdate {
			status.Infof(ctx, "%s: Skipped update to version %s", relDir, latestVersion)
			continue
		}

		status.Infof(ctx, "%s: Chart %s (old version %s) has new version %s available", relDir, hr.Chart.GetChartName(), currentVersion, latestVersion)

		if !cmd.Upgrade {
			continue
		}

		if cmd.Interactive {
			if !prompts.AskForConfirmation(ctx, fmt.Sprintf("%s: Do you want to upgrade Chart %s to version %s?",
				relDir, hr.Chart.GetChartName(), latestVersion)) {
				continue
			}
		}

		oldVersion := currentVersion
		hr.Config.ChartVersion = latestVersion
		err = hr.Save()
		if err != nil {
			return err
		}
		status.Infof(ctx, "%s: Updated Chart version to %s", relDir, latestVersion)

		k := helmUpgradeKey{
			chartDir:   cd,
			oldVersion: oldVersion,
			newVersion: latestVersion,
		}
		upgrades[k] = append(upgrades[k], hr)
	}

	for k, hrs := range upgrades {
		err = cmd.pullAndCommit(ctx, projectDir, baseChartsDir, gitRootPath, hrs, k.oldVersion, helmAuthProvider, ociAuthProvider, gitRp, ociRp)
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

func (cmd *helmUpdateCmd) pullAndCommit(ctx context.Context, projectDir string, baseChartsDir string, gitRootPath string, hrs []*helm.Release, oldVersion string, helmAuthProvider helmauth.HelmAuthProvider, ociAuthProvider *ociauth.OciAuthProviders, gitRp *repocache.GitRepoCache, ociRp *repocache.OciRepoCache) error {
	chart := hrs[0].Chart
	newVersion := hrs[0].Config.ChartVersion

	s := status.Startf(ctx, "Upgrading Chart %s from version %s to %s", chart.GetChartName(), oldVersion, newVersion)
	defer s.Failed()

	doError := func(err error) error {
		s.FailedWithMessage(err.Error())
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

	_, err = doHelmPull(ctx, projectDir, helmAuthProvider, ociAuthProvider, gitRp, ociRp, false, false)
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

		s.UpdateAndInfoFallbackf("Committed helm chart %s with version %s", chart.GetChartName(), newVersion)
	}
	s.Success()

	return nil
}

type helmUpgradeKey struct {
	chartDir   string
	oldVersion string
	newVersion string
}
