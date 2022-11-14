package commands

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	git2 "github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"io/fs"
	"os"
	"path/filepath"
)

type helmUpdateCmd struct {
	args.HelmCredentials

	Upgrade bool `group:"misc" help:"Write new versions into helm-chart.yaml and perform helm-pull afterwards"`
	Commit  bool `group:"misc" help:"Create a git commit for every updated chart"`
}

func (cmd *helmUpdateCmd) Help() string {
	return `Optionally performs the actual upgrade and/or add a commit to version control.`
}

func (cmd *helmUpdateCmd) Run() error {
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

			s := status.Start(cliCtx, "%s: Checking for updates", statusPrefix)
			defer s.Failed()

			chart, err := deployment.NewHelmChart(p)
			if err != nil {
				s.Update("%s: Error while loading helm-chart.yaml: %v", statusPrefix, err)
				return err
			}

			creds := cmd.HelmCredentials.FindCredentials(*chart.Config.Repo, chart.Config.CredentialsId)
			if chart.Config.CredentialsId != nil && creds == nil {
				err := fmt.Errorf("%s: No credentials provided", statusPrefix)
				s.FailedWithMessage(err.Error())
				return err
			}
			chart.SetCredentials(creds)

			newVersion, updated, err := chart.CheckUpdate()
			if err != nil {
				return err
			}
			if !updated {
				s.Update("%s: Version %s is already up-to-date.", statusPrefix, *chart.Config.ChartVersion)
				s.Success()
				return nil
			}
			msg := fmt.Sprintf("%s: Chart has new version %s available. Old version is %s.", statusPrefix, newVersion, *chart.Config.ChartVersion)
			if chart.Config.SkipUpdate {
				msg += " skipUpdate is set to true."
			}
			s.Update(msg)

			if !cmd.Upgrade {
				s.Success()
			} else {
				if chart.Config.SkipUpdate {
					s.Update("%s: NOT upgrading chart as skipUpdate was set to true", statusPrefix)
					s.Success()
					return nil
				}

				oldVersion := *chart.Config.ChartVersion
				chart.Config.ChartVersion = &newVersion
				err = chart.Save()
				if err != nil {
					return err
				}

				chartsDir := filepath.Join(filepath.Dir(p), "charts")

				// we need to list all files contained inside the charts dir BEFORE doing the pull, so that we later
				// know what got deleted
				gitFiles := make(map[string]bool)
				gitFiles[p] = true
				err = filepath.WalkDir(chartsDir, func(p string, d fs.DirEntry, err error) error {
					if !d.IsDir() {
						gitFiles[p] = true
					}
					return nil
				})
				if err != nil {
					return err
				}

				s.Update("%s: Pulling new version", statusPrefix)
				defer s.Failed()

				err = chart.Pull(cliCtx)
				if err != nil {
					return err
				}

				// and now list all files again to catch all new files
				err = filepath.WalkDir(chartsDir, func(p string, d fs.DirEntry, err error) error {
					if !d.IsDir() {
						gitFiles[p] = true
					}
					return nil
				})
				if err != nil {
					return err
				}

				if cmd.Commit {
					commitMsg := fmt.Sprintf("Updated helm chart %s from %s to %s", filepath.Dir(p), oldVersion, newVersion)

					s.Update(fmt.Sprintf("%s: Updating chart from %s to %s", statusPrefix, oldVersion, newVersion))

					r, err := git.PlainOpen(gitRootPath)
					if err != nil {
						return err
					}
					wt, err := r.Worktree()
					if err != nil {
						return err
					}
					for p := range gitFiles {
						absPath, err := filepath.Abs(filepath.Join(cwd, p))
						if err != nil {
							return err
						}
						relToGit, err := filepath.Rel(gitRootPath, absPath)
						if err != nil {
							return err
						}
						_, err = wt.Add(relToGit)
						if err != nil {
							return err
						}
					}
					_, err = wt.Commit(commitMsg, &git.CommitOptions{})
					if err != nil {
						return err
					}
				}
				s.Success()
			}
		}
		return nil
	})
	return err
}
