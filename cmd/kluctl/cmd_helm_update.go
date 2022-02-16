package main

import (
	"fmt"
	"github.com/codablock/kluctl/cmd/kluctl/args"
	"github.com/codablock/kluctl/pkg/deployment"
	"github.com/go-git/go-git/v5"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io/fs"
	"path"
	"path/filepath"
)

var (
	upgrade bool
	commit  bool
)

func runCmdHelmUpdate(cmd *cobra.Command, args_ []string) error {
	rootPath := "."
	if args.LocalDeployment != "" {
		rootPath = args.LocalDeployment
	}
	err := filepath.WalkDir(rootPath, func(p string, d fs.DirEntry, err error) error {
		fname := path.Base(p)
		if fname == "helm-chart.yml" {
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

			if upgrade {
				if chart.Config.SkipUpdate != nil && !*chart.Config.SkipUpdate {
					log.Infof("NOT upgrading chart %s as skipUpdate was set to true", p)
					return nil
				}

				oldVersion := *chart.Config.ChartVersion
				chart.Config.ChartVersion = &newVersion
				err = chart.Save()
				if err != nil {
					return err
				}

				chartsDir := path.Join(path.Dir(p), "charts")

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

				if commit {
					msg := fmt.Sprintf("Updated helm chart %s from %s to %s", path.Dir(p), oldVersion, newVersion)
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

func init() {
	var cmd = &cobra.Command{
		Use:   "helm-update",
		Short: "Recursively searches for `helm-chart.yml` files and checks for new available versions",
		Long: "Recursively searches for `helm-chart.yml` files and checks for new available versions.\n\n" +
			"Optionally performs the actual upgrade and/or add a commit to version control.",
		RunE: runCmdHelmUpdate,
	}
	cmd.Flags().StringVar(&args.LocalDeployment, "local-deployment", "", "Local deployment directory. Defaults to current directory")
	cmd.Flags().BoolVar(&upgrade, "upgrade", false, "Write new versions into helm-chart.yml and perform helm-pull afterwards")
	cmd.Flags().BoolVar(&commit, "commit", false, "Create a git commit for every updated chart")
	err := viper.BindPFlags(cmd.Flags())
	if err != nil {
		panic(err)
	}

	rootCmd.AddCommand(cmd)
}
