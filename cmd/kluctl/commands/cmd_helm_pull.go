package commands

import (
	"github.com/codablock/kluctl/pkg/deployment"
	log "github.com/sirupsen/logrus"
	"io/fs"
	"path/filepath"
)

type helmPullCmd struct {
	LocalDeployment string `group:"project" help:"Local deployment directory. Defaults to current directory"`
}

func (cmd *helmPullCmd) Help() string {
	return `The Helm charts are stored under the sub-directory 'charts/<chart-name>' next to the
'helm-chart.yml'. These Helm charts are meant to be added to version control so that
pulling is only needed when really required (e.g. when the chart version changes).`
}

func (cmd *helmPullCmd) Run() error {
	rootPath := "."
	if cmd.LocalDeployment != "" {
		rootPath = cmd.LocalDeployment
	}
	err := filepath.WalkDir(rootPath, func(p string, d fs.DirEntry, err error) error {
		fname := filepath.Base(p)
		if fname == "helm-chart.yml" || fname == "helm-chart.yaml" {
			log.Infof("Pulling for %s", p)
			chart, err := deployment.NewHelmChart(p)
			if err != nil {
				return err
			}
			err = chart.Pull()
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}
