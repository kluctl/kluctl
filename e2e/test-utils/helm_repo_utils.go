package test_utils

import (
	"github.com/kluctl/kluctl/v2/e2e/test-utils/test-helm-chart"
	"github.com/kluctl/kluctl/v2/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func CreateHelmDir(t *testing.T, name string, version string, dest string) {
	err := fs.WalkDir(test_resources.HelmChartFS, ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return os.MkdirAll(filepath.Join(dest, path), 0o700)
		} else {
			b, err := test_resources.HelmChartFS.ReadFile(path)
			if err != nil {
				return err
			}
			return os.WriteFile(filepath.Join(dest, path), b, 0o600)
		}
	})
	if err != nil {
		t.Fatal(err)
	}

	c, err := uo.FromFile(filepath.Join(dest, "Chart.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	_ = c.SetNestedField(name, "name")
	_ = c.SetNestedField(version, "version")

	err = yaml.WriteYamlFile(filepath.Join(dest, "Chart.yaml"), c)
	if err != nil {
		t.Fatal(err)
	}
}

func createHelmPackage(t *testing.T, name string, version string) string {
	tmpDir := t.TempDir()

	CreateHelmDir(t, name, version, tmpDir)

	settings := cli.New()
	client := action.NewPackage()
	client.Destination = tmpDir
	valueOpts := &values.Options{}
	p := getter.All(settings)
	vals, err := valueOpts.MergeValues(p)
	retName, err := client.Run(tmpDir, vals)
	if err != nil {
		t.Fatal(err)
	}

	return retName
}
