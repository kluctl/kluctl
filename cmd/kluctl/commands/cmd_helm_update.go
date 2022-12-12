package commands

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/hashicorp/go-multierror"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	git2 "github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/helm"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"golang.org/x/sync/semaphore"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
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

type updatedChart struct {
	charts     []*helm.HelmChart
	newVersion string
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

	baseChartsDir := filepath.Join(projectDir, ".helm-charts")

	var errs *multierror.Error
	var wg sync.WaitGroup
	var mutex sync.Mutex
	sem := semaphore.NewWeighted(8)

	chartsToCheck := map[string]*updatedChart{}

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

		key := buildKey(chart, chart.Config.UpdateConstraints, nil)
		if uc2, ok := chartsToCheck[key]; ok {
			uc2.charts = append(uc2.charts, chart)
		} else {
			chartsToCheck[key] = &updatedChart{
				charts: []*helm.HelmChart{chart},
			}
		}
		return nil
	})

	toKeep := map[string]bool{}
	chartsToPull := map[string]*updatedChart{}

	for _, uc := range chartsToCheck {
		uc := uc
		utils.GoLimitedMultiError(ctx, sem, &errs, &mutex, &wg, func() error {
			newVersion, err := cmd.doQueryLatestVersion(ctx, uc.charts[0])
			if err != nil {
				return err
			}

			mutex.Lock()
			defer mutex.Unlock()

			key := buildKey(uc.charts[0], &newVersion, nil)
			toKeep[key] = true

			uc2, ok := chartsToPull[key]
			if !ok {
				uc2 = &updatedChart{
					newVersion: newVersion,
				}
				chartsToPull[key] = uc2
			}

			for _, chart := range uc.charts {
				updated := newVersion != *chart.Config.ChartVersion
				if updated && chart.Config.SkipUpdate {
					status.Info(ctx, "%s: Skipping update to version %s", chart.GetChartName(), newVersion)
					updated = false
				}
				if !updated {
					toKeep[buildKey(chart, chart.Config.ChartVersion, nil)] = true
					continue
				}
				if len(uc2.charts) == 0 {
					status.Info(ctx, "%s: Chart has new version %s", chart.GetChartName(), newVersion)
				}
				uc2.charts = append(uc2.charts, chart)
			}

			if len(uc2.charts) == 0 {
				delete(chartsToPull, key)
			}

			return nil
		})
	}
	wg.Wait()
	if errs.ErrorOrNil() != nil {
		return errs
	}

	if !cmd.Upgrade {
		return errs.ErrorOrNil()
	}

	if cmd.Interactive {
		sem = semaphore.NewWeighted(1)
	}

	for _, uc := range chartsToPull {
		uc := uc

		utils.GoLimitedMultiError(ctx, sem, &errs, &mutex, &wg, func() error {
			if cmd.Interactive {
				statusPrefix := uc.charts[0].GetChartName()
				if !status.AskForConfirmation(ctx, fmt.Sprintf("%s: Do you want to upgrade Chart %s to version %s?",
					statusPrefix, uc.charts[0].GetChartName(), uc.newVersion)) {
					return nil
				}
			}

			err := cmd.pullAndCommitCharts(ctx, gitRootPath, uc, toKeep, &mutex)
			if err != nil {
				return err
			}
			return nil
		})
	}
	wg.Wait()

	if !cmd.Commit {
		return errs.ErrorOrNil()
	}

	return errs.ErrorOrNil()
}

func buildKey(chart *helm.HelmChart, v1 *string, v2 *string) string {
	key := chart.GetChartDir(false)
	if v1 != nil {
		key += " / " + *v1
	}
	if v2 != nil {
		key += " / " + *v2
	}
	return key
}

func (cmd *helmUpdateCmd) doQueryLatestVersion(ctx context.Context, chart *helm.HelmChart) (string, error) {
	statusPrefix := chart.GetChartName()

	statusText := fmt.Sprintf("%s: Querying latest version", statusPrefix)
	if chart.Config.UpdateConstraints != nil {
		statusText = fmt.Sprintf("%s: Querying latest version with constraints '%s'", statusPrefix, *chart.Config.UpdateConstraints)
	}

	s := status.Start(ctx, statusText)
	defer s.Failed()

	doError := func(err error) (string, error) {
		s.FailedWithMessage("%s: %s", statusPrefix, err.Error())
		return "", err
	}

	latestVersion, err := chart.QueryLatestVersion(ctx)
	if err != nil {
		return doError(err)
	}
	s.Success()

	return latestVersion, nil
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
		st, err := d.Info()
		if err != nil {
			return err
		}
		m[relToGit] = st
		return nil
	})
	if os.IsNotExist(err) {
		err = nil
	}
	return err
}

func (cmd *helmUpdateCmd) pullAndCommitCharts(ctx context.Context, gitRootPath string, uc *updatedChart, toKeep map[string]bool, mutex *sync.Mutex) error {
	statusPrefix := uc.charts[0].GetChartName()

	s := status.Start(ctx, "%s: Pulling Chart with version %s", statusPrefix, uc.newVersion)
	defer s.Failed()

	doError := func(err error) error {
		s.FailedWithMessage("%s: %s", statusPrefix, err.Error())
		return err
	}

	var toAdd []string
	var oldVersions []string
	var oldChartDirs []string

	// we need to list all files contained inside the charts dir BEFORE doing the pull, so that we later
	// know what got deleted
	files := map[string]os.FileInfo{}

	for _, chart := range uc.charts {
		oldChartDirs = append(oldChartDirs, chart.GetChartDir(true))
		oldVersions = append(oldVersions, *chart.Config.ChartVersion)

		chart.Config.ChartVersion = &uc.newVersion
		err := chart.Save()
		if err != nil {
			return doError(err)
		}

		// add helm-chart.yaml
		relToGit, err := filepath.Rel(gitRootPath, chart.ConfigFile)
		if err != nil {
			return doError(err)
		}
		toAdd = append(toAdd, relToGit)

		err = cmd.collectFiles(gitRootPath, chart.GetDeprecatedChartDir(), files)
		if err != nil {
			return doError(err)
		}
	}

	err := cmd.collectFiles(gitRootPath, uc.charts[0].GetChartDir(true), files)
	if err != nil {
		return doError(err)
	}

	err = uc.charts[0].Pull(ctx)
	if err != nil {
		return doError(err)
	}

	relToGit, err := filepath.Rel(gitRootPath, uc.charts[0].GetChartDir(true))
	if err != nil {
		return doError(err)
	}
	toAdd = append(toAdd, relToGit)

	// delete old versions
	for i, chart := range uc.charts {
		deleteOldVersion := !toKeep[buildKey(chart, &oldVersions[i], nil)]
		if deleteOldVersion {
			err = os.RemoveAll(oldChartDirs[i])
			if err != nil {
				return doError(err)
			}
		}

		err = os.RemoveAll(chart.GetDeprecatedChartDir())
		if err != nil {
			return doError(err)
		}
	}

	// figure out what got deleted
	for p, oldSt := range files {
		st, err := os.Lstat(filepath.Join(gitRootPath, p))
		if err != nil {
			if os.IsNotExist(err) {
				toAdd = append(toAdd, p)
				continue
			}
			return doError(err)
		}
		if st.Mode() == oldSt.Mode() && st.ModTime() == oldSt.ModTime() && st.Size() == oldSt.Size() {
			continue
		}
		toAdd = append(toAdd, p)
	}

	s.UpdateAndInfoFallback("%s: Committing chart", statusPrefix)

	mutex.Lock()
	defer mutex.Unlock()

	r, err := git.PlainOpen(gitRootPath)
	if err != nil {
		return doError(err)
	}
	wt, err := r.Worktree()
	if err != nil {
		return doError(err)
	}

	for _, p := range toAdd {
		// we have to retry a few times as Add() might fail with "no such file or directly"
		// This is because it internally tries to get the git status, which fails if files are added/deleted in
		// parallel by another goroutine (we're pulling in parallel). We're guarding the repo via the mutex from above
		// so this is actually safe.
		for i := 0; i < 10; i++ {
			_, err = wt.Add(p)
			if err == nil || !os.IsNotExist(err) {
				break
			}
			// let's have some randomness in waiting time to ensure we don't run into the same problem again and again
			s := time.Duration(rand.Intn(10) + 10)
			time.Sleep(s * time.Millisecond)
		}
		if err != nil {
			return doError(fmt.Errorf("failed to add %s to git index: %w", p, err))
		}
	}

	commitMsg := fmt.Sprintf("Updated helm chart %s to version %s", statusPrefix, uc.newVersion)
	_, err = wt.Commit(commitMsg, &git.CommitOptions{})
	if err != nil {
		return doError(fmt.Errorf("failed to commit: %w", err))
	}

	s.Update("%s: Committed helm chart with version %s", statusPrefix, uc.newVersion)
	s.Success()

	return nil
}
