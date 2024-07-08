package helm

import (
	"github.com/kluctl/kluctl/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"os"
	"path/filepath"
	"time"
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
	return false, false, version, nil
}
