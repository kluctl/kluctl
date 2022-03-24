package commands

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/kluctl/kluctl/pkg/deployment"
	log "github.com/sirupsen/logrus"
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
	err := filepath.WalkDir(rootPath, func(p string, d fs.DirEntry, err error) error {
		fname := filepath.Base(p)
		if fname == "helm-chart.yml" || fname == "helm-chart.yaml" {
			chart, err := deployment.NewHelmChart(p)
			if err != nil {
				return err
			}
			newVersion, updated, err := chart.CheckUpdate()
			if err != nil {
				return err
			}
			if !updated {
				return nil
			}
			log.Infof("Chart %s has new version %s available. Old version is %s.", p, newVersion, *chart.Config.ChartVersion)

			if cmd.Upgrade {
				if chart.Config.SkipUpdate != nil && *chart.Config.SkipUpdate {
					log.Infof("NOT upgrading chart %s as skipUpdate was set to true", p)
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

				log.Infof("Pulling for %s", p)
				err = chart.Pull()
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
					msg := fmt.Sprintf("Updated helm chart %s from %s to %s", filepath.Dir(p), oldVersion, newVersion)
					log.Infof("Committing: %s", msg)
					r, err := git.PlainOpen(rootPath)
					if err != nil {
						return err
					}
					wt, err := r.Worktree()
					if err != nil {
						return err
					}
					for p := range gitFiles {
						_, err = wt.Add(p)
						if err != nil {
							return err
						}
					}
					_, err = wt.Commit(msg, &git.CommitOptions{})
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	})
	return err
}
