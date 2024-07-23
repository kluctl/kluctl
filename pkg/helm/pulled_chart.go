package helm

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/kluctl/kluctl/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
)

type PulledChart struct {
	chart   *Chart
	version string
	dir     string
	isTmp   bool
}

func NewPulledChart(chart *Chart, version string, dir string, isTmp bool) *PulledChart {
	pc := &PulledChart{
		chart:   chart,
		version: version,
		dir:     dir,
		isTmp:   isTmp,
	}
	return pc
}

func (pc *PulledChart) CheckExists() bool {
	if !utils.IsDirectory(pc.dir) {
		return false
	}
	return true
}

func (pc *PulledChart) CheckNeedsPull() (bool, bool, string, error) {
	if !utils.IsDirectory(pc.dir) {
		return true, false, "", nil
	}
	if pc.chart.IsRegistryChart() {
		chartYamlPath := yaml.FixPathExt(filepath.Join(pc.dir, "Chart.yaml"))
		st, err := os.Stat(chartYamlPath)
		if err != nil {
			if os.IsNotExist(err) {
				return true, false, "", nil
			}
			return false, false, "", err
		}

		if pc.isTmp && time.Now().Sub(st.ModTime()) >= time.Hour*24 {
			// MacOS will delete tmp files after 3 days, so lets be safe and re-pull every day
			return true, false, "", nil
		}

		chartYaml, err := uo.FromFile(chartYamlPath)
		if err != nil {
			return false, false, "", err
		}

		version, _, _ := chartYaml.GetNestedString("version")

		if version != pc.version {
			return true, true, version, nil
		}
	}

	if pc.chart.IsRepositoryChart() {
		// Update git cache and check if git-info differs
		// If something is missing, re-pull
		m, err := pc.chart.gitRp.GetEntry(pc.chart.git.Url.String())
		if err != nil {
			return true, false, "", err
		}
		err = m.Update()
		if err != nil {
			return true, false, "", err
		}
		_, gitInfo, err := m.GetClonedDir(pc.chart.git.Ref)
		if err != nil {
			return true, false, "", err
		}
		gif, err := os.ReadFile(filepath.Join(pc.dir, ".git-info"))
		if err != nil {
			return true, false, "", nil
		}
		out, err := json.Marshal(gitInfo)
		if err != nil {
			return true, false, "", nil

		}
		if err != nil {
			return true, false, "", err
		}
		if bytes.Equal(out, gif) {
			return false, false, "", nil
		} else {
			return true, true, gitInfo.CheckedOutCommit, nil
		}

	}

	return false, false, "", nil
}
