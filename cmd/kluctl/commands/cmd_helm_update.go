package commands

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	git2 "github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"io/fs"
	"path/filepath"
)

type helmUpdateCmd struct {
	LocalDeployment string `group:"project" help:"Local deployment directory. Defaults to current directory"`
	Upgrade         bool   `group:"misc" help:"Write new versions into helm-chart.yml and perform helm-pull afterwards"`
	Commit          bool   `group:"misc" help:"Create a git commit for every updated chart"`
}

func (cmd *helmUpdateCmd) Help() string {
	return `Optionally performs the actual upgrade and/or add a commit to version control.`
}

func (cmd *helmUpdateCmd) Run() error {
	rootPath := "."
	if cmd.LocalDeployment != "" {
		rootPath = cmd.LocalDeployment
	}
	gitRootPath, err := git2.DetectGitRepositoryRoot(rootPath)
	if err != nil {
		return err
	}

	err = filepath.WalkDir(rootPath, func(p string, d fs.DirEntry, err error) error {
		fname := filepath.Base(p)
		if fname == "helm-chart.yml" || fname == "helm-chart.yaml" {
			chart, err := deployment.NewHelmChart(p)
			if err != nil {
				return err
			}

			statusPrefix := filepath.Base(filepath.Dir(p))
			s := status.Start(cliCtx, "%s: Checking for updates", statusPrefix)
			defer s.Failed()

			skipUpdate := chart.Config.SkipUpdate != nil && *chart.Config.SkipUpdate

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
			if skipUpdate {
				msg += " skipUpdate is set to true."
			}
			s.Update(msg)

			if !cmd.Upgrade {
				s.Success()
			} else {
				if skipUpdate {
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
						absPath, err := filepath.Abs(filepath.Join(rootPath, p))
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
