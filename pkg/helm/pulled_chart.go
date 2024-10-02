package helm

import (
	"bytes"
	"os"
	"path/filepath"
	"time"

	"github.com/kluctl/kluctl/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
)

type PulledChart struct {
	chart   *Chart
	version ChartVersion
	dir     string
	isTmp   bool
}

func NewPulledChart(chart *Chart, version ChartVersion, dir string, isTmp bool) *PulledChart {
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

func (pc *PulledChart) CheckNeedsPull() (bool, bool, ChartVersion, error) {
	var nullVersion ChartVersion

	if !utils.IsDirectory(pc.dir) {
		return true, false, nullVersion, nil
	}
	if pc.chart.IsRegistryChart() {
		chartYamlPath := yaml.FixPathExt(filepath.Join(pc.dir, "Chart.yaml"))
		st, err := os.Stat(chartYamlPath)
		if err != nil {
			if os.IsNotExist(err) {
				return true, false, nullVersion, nil
			}
			return false, false, nullVersion, err
		}

		if pc.isTmp && time.Now().Sub(st.ModTime()) >= time.Hour*24 {
			// MacOS will delete tmp files after 3 days, so lets be safe and re-pull every day
			return true, false, nullVersion, nil
		}

		chartYaml, err := uo.FromFile(chartYamlPath)
		if err != nil {
			return false, false, nullVersion, err
		}

		version, _, _ := chartYaml.GetNestedString("version")
		newVersion := ChartVersion{
			Version: &version,
		}

		if newVersion.String() != pc.version.String() {
			return true, true, newVersion, nil
		}
	}

	if pc.chart.IsGitRepositoryChart() {
		// Update git cache and check if git-info differs
		// If something is missing, re-pull
		m, err := pc.chart.gitRp.GetEntry(pc.chart.gitUrl.String())
		if err != nil {
			return true, false, nullVersion, err
		}
		_, gitInfo, err := m.GetClonedDir(pc.version.GitRef)
		if err != nil {
			return true, false, nullVersion, err
		}
		gif, err := os.ReadFile(filepath.Join(pc.dir, ".git-info.yaml"))
		if err != nil {
			return true, false, nullVersion, nil
		}
		out, err := yaml.WriteYamlBytes(gitInfo)
		if err != nil {
			return true, false, nullVersion, err
		}
		if bytes.Equal(out, gif) {
			return false, false, nullVersion, nil
		} else {
			newVersion := ChartVersion{
				GitRef: &gitInfo.CheckedOutRef,
			}
			return true, true, newVersion, nil
		}

	}

	return false, false, nullVersion, nil
}
