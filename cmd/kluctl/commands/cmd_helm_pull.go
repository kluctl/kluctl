package commands

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

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

type helmPullCmd struct {
	args.ProjectDir
	args.HelmCredentials
	args.RegistryCredentials
}

func (cmd *helmPullCmd) Help() string {
	return `Kluctl requires Helm Charts to be pre-pulled by default, which is handled by this command. It will collect
all required Charts and versions and pre-pull them into .helm-charts. To disable pre-pulling for individual charts,
set 'skipPrePull: true' in helm-chart.yaml.`
}

func (cmd *helmPullCmd) Run(ctx context.Context) error {
	projectDir, err := cmd.ProjectDir.GetProjectDir()
	if err != nil {
		return err
	}

	if !yaml.Exists(filepath.Join(projectDir, ".kluctl.yaml")) && !yaml.Exists(filepath.Join(projectDir, ".kluctl-library.yaml")) {
		return fmt.Errorf("helm-pull can only be used on the root of a Kluctl project that must have a .kluctl.yaml or .kluctl-library.yaml file")
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

	_, err = doHelmPull(ctx, projectDir, helmAuthProvider, ociAuthProvider, gitRp, ociRp, false, true)
	return err
}

func cleanupUnusedCharts(ctx context.Context, versionsToPull map[string]bool, dryRun bool, chartsDir string) (int, error) {
	actions := 0
	des, err := os.ReadDir(chartsDir)
	if err != nil && !os.IsNotExist(err) {
		return actions, err
	}
	for _, de := range des {
		if !de.IsDir() {
			continue
		}
		if _, ok := versionsToPull[de.Name()]; !ok {
			actions++
			if !dryRun {
				status.Infof(ctx, "Removing unused Chart with version or ref %s", de.Name())
				err = os.RemoveAll(filepath.Join(chartsDir, de.Name()))
				if err != nil {
					return actions, err
				}
			}
		}
	}
	return actions, nil
}

func doHelmPull(ctx context.Context, projectDir string, helmAuthProvider helmauth.HelmAuthProvider, ociAuthProvider ociauth.OciAuthProvider, gitRp *repocache.GitRepoCache, ociRp *repocache.OciRepoCache, dryRun bool, force bool) (int, error) {
	actions := 0

	baseChartsDir := filepath.Join(projectDir, ".helm-charts")

	releases, charts, err := loadHelmReleases(ctx, projectDir, baseChartsDir, helmAuthProvider, ociAuthProvider, gitRp, ociRp)
	if err != nil {
		return actions, err
	}

	g := utils.NewGoHelper(ctx, 8)

	for _, chart := range charts {
		chart := chart
		statusPrefix := chart.GetChartName()
		versionsToPull := map[string]bool{}
		for _, hr := range releases {
			if hr.Config.SkipPrePull {
				continue
			}
			if hr.Chart == chart {
				if hr.Chart.IsRegistryChart() {
					versionsToPull[hr.Config.ChartVersion] = true
				}
				if hr.Chart.IsGitRepositoryChart() {
					ref, _, err := hr.Config.GetGitRef()
					if err != nil {
						return actions, err
					}
					versionsToPull[ref] = true
				}
			}
		}

		chartsDir, err := chart.BuildPulledChartDir(baseChartsDir)
		if err != nil {
			return actions, err
		}
		cleanupActions, err := cleanupUnusedCharts(ctx, versionsToPull, dryRun, chartsDir)
		actions += cleanupActions

		for version, _ := range versionsToPull {
			version := version

			if yaml.Exists(filepath.Join(chartsDir, version, "Chart.yaml")) && !force {
				continue
			}

			actions++

			if dryRun {
				continue
			}
			g.RunE(func() error {
				s := status.Startf(ctx, "%s: Pulling Chart with version %s", statusPrefix, version)
				defer s.Failed()

				_, err := chart.PullInProject(ctx, baseChartsDir, version)
				if err != nil {
					s.FailedWithMessagef("%s: %s", statusPrefix, err.Error())
					return err
				}

				s.Success()
				return nil
			})
		}
	}
	g.Wait()

	if g.ErrorOrNil() != nil {
		return actions, fmt.Errorf("command failed")
	}

	return actions, nil
}

func loadHelmReleases(ctx context.Context, projectDir string, baseChartsDir string, helmAuthProvider helmauth.HelmAuthProvider, ociAuthProvider ociauth.OciAuthProvider, gitRp *repocache.GitRepoCache, ociRp *repocache.OciRepoCache) ([]*helm.Release, []*helm.Chart, error) {
	var releases []*helm.Release
	chartsMap := make(map[string]*helm.Chart)
	err := filepath.WalkDir(projectDir, func(p string, d fs.DirEntry, err error) error {
		fname := filepath.Base(p)
		if fname != "helm-chart.yml" && fname != "helm-chart.yaml" {
			return nil
		}

		relDir, err := filepath.Rel(projectDir, filepath.Dir(p))
		if err != nil {
			return err
		}

		hr, err := helm.NewRelease(ctx, projectDir, relDir, p, baseChartsDir, helmAuthProvider, ociAuthProvider, gitRp, ociRp)
		if err != nil {
			return err
		}

		if hr.Chart.IsLocalChart() {
			return nil
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
