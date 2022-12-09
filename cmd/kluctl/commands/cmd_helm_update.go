package commands

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/hashicorp/go-multierror"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	git2 "github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"golang.org/x/sync/semaphore"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type helmUpdateCmd struct {
	args.HelmCredentials

	Upgrade bool `group:"misc" help:"Write new versions into helm-chart.yaml and perform helm-pull afterwards"`
	Commit  bool `group:"misc" help:"Create a git commit for every updated chart"`

	Interactive bool `group:"misc" short:"i" help:"Ask for every Helm Chart if it should be upgraded."`
}

func (cmd *helmUpdateCmd) Help() string {
	return `Optionally performs the actual upgrade and/or add a commit to version control.`
}

func (cmd *helmUpdateCmd) Run(ctx context.Context) error {
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

	type updatedChart struct {
		path        string
		chart       *deployment.HelmChart
		newVersion  string
		oldVersion  string
		pullSuccess bool
	}
	var updatedCharts []*updatedChart

	err = filepath.WalkDir(cwd, func(p string, d fs.DirEntry, err error) error {
		fname := filepath.Base(p)
		if fname != "helm-chart.yml" && fname != "helm-chart.yaml" {
			return nil
		}

		utils.GoLimitedMultiError(ctx, sem, &errs, &mutex, &wg, func() error {
			chart, newVersion, updated, err := cmd.doCheckUpdate(ctx, gitRootPath, p)
			if err != nil {
				return err
			}

			mutex.Lock()
			defer mutex.Unlock()

			if !chart.Config.SkipUpdate && updated {
				updatedCharts = append(updatedCharts, &updatedChart{
					path:       p,
					chart:      chart,
					newVersion: newVersion,
					oldVersion: *chart.Config.ChartVersion,
				})
			}
			return nil
		})
		return nil
	})
	wg.Wait()
	if err != nil {
		errs = multierror.Append(errs, err)
		return errs.ErrorOrNil()
	}

	if !cmd.Upgrade {
		return errs.ErrorOrNil()
	}

	if cmd.Interactive {
		sem = semaphore.NewWeighted(1)
	}

	for _, uc := range updatedCharts {
		uc := uc

		utils.GoLimitedMultiError(ctx, sem, &errs, &mutex, &wg, func() error {
			if cmd.Interactive {
				statusPrefix, _ := filepath.Rel(gitRootPath, filepath.Dir(uc.path))
				if !status.AskForConfirmation(ctx, fmt.Sprintf("%s: Do you want to upgrade Chart %s from version %s to %s?",
					statusPrefix, uc.chart.GetChartName(), uc.oldVersion, uc.newVersion)) {
					return nil
				}
			}

			err := cmd.pullAndCommitChart(ctx, gitRootPath, uc.chart, uc.oldVersion, uc.newVersion, &mutex)
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

func (cmd *helmUpdateCmd) doCheckUpdate(ctx context.Context, gitRootPath string, p string) (*deployment.HelmChart, string, bool, error) {
	statusPrefix, err := filepath.Rel(gitRootPath, filepath.Dir(p))
	if err != nil {
		return nil, "", false, err
	}

	s := status.Start(ctx, "%s: Checking for updates", statusPrefix)
	doError := func(err error) (*deployment.HelmChart, string, bool, error) {
		s.FailedWithMessage("%s: %s", statusPrefix, err.Error())
		return nil, "", false, err
	}

	chart, err := deployment.NewHelmChart(p)
	if err != nil {
		return doError(err)
	}

	chart.SetCredentials(&cmd.HelmCredentials)

	newVersion, updated, err := chart.CheckUpdate(ctx)
	if err != nil {
		return doError(err)
	}
	if !updated {
		s.UpdateAndInfoFallback("%s: Version %s is already up-to-date.", statusPrefix, *chart.Config.ChartVersion)
	} else {
		msg := fmt.Sprintf("%s: Chart has new version %s available. Old version is %s.", statusPrefix, newVersion, *chart.Config.ChartVersion)
		if chart.Config.SkipUpdate {
			msg += " skipUpdate is set to true."
		}
		s.UpdateAndInfoFallback(msg)
	}
	s.Success()

	return chart, newVersion, updated, nil
}

func (cmd *helmUpdateCmd) pullAndCommitChart(ctx context.Context, gitRootPath string, chart *deployment.HelmChart, oldVersion string, newVersion string, mutex *sync.Mutex) error {
	statusPrefix, err := filepath.Rel(gitRootPath, filepath.Dir(chart.ConfigFile))
	if err != nil {
		return err
	}

	s := status.Start(ctx, "%s: Pulling Chart", statusPrefix)
	defer s.Failed()

	chart.Config.ChartVersion = &newVersion
	err = chart.Save()
	if err != nil {
		return err
	}

	// we need to list all files contained inside the charts dir BEFORE doing the pull, so that we later
	// know what got deleted
	oldFiles := map[string]bool{}
	err = filepath.WalkDir(chart.GetChartDir(), func(p string, d fs.DirEntry, err error) error {
		if d == nil || d.IsDir() {
			return nil
		}
		relToGit, err := filepath.Rel(gitRootPath, p)
		if err != nil {
			return err
		}
		oldFiles[relToGit] = true
		return nil
	})
	if err != nil {
		return err
	}

	err = doPull(ctx, statusPrefix, chart.ConfigFile, cmd.HelmCredentials, s)
	if err != nil {
		return err
	}

	var toAdd []string
	relToGit, err := filepath.Rel(gitRootPath, chart.ConfigFile)
	if err != nil {
		return err
	}
	toAdd = append(toAdd, relToGit)

	relToGit, err = filepath.Rel(gitRootPath, chart.GetChartDir())
	if err != nil {
		return err
	}
	toAdd = append(toAdd, relToGit)

	// figure out what got deleted
	for p, _ := range oldFiles {
		if !utils.IsFile(filepath.Join(gitRootPath, p)) {
			toAdd = append(toAdd, p)
		}
	}

	s.UpdateAndInfoFallback("%s: Committing chart", statusPrefix)

	mutex.Lock()
	defer mutex.Unlock()

	r, err := git.PlainOpen(gitRootPath)
	if err != nil {
		return err
	}
	wt, err := r.Worktree()
	if err != nil {
		return err
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
			return fmt.Errorf("failed to add %s to git index: %w", p, err)
		}
	}

	commitMsg := fmt.Sprintf("Updated helm chart %s from %s to %s", statusPrefix, oldVersion, newVersion)
	_, err = wt.Commit(commitMsg, &git.CommitOptions{})
	if err != nil {
		return fmt.Errorf("failed to commit: %w", err)
	}

	s.Update("%s: Committed helm chart with version %s", statusPrefix, newVersion)
	s.Success()

	return nil
}
