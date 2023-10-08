package test_utils

import (
	"github.com/kluctl/kluctl/v2/e2e/test-utils/test-helm-chart"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	cp "github.com/otiai10/copy"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/pusher"
	registry2 "helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/uploader"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

type RepoChart struct {
	ChartName string
	Version   string
}

func CreateHelmRepo(t *testing.T, charts []RepoChart, username string, password string) string {
	tmpDir := t.TempDir()

	for _, c := range charts {
		tgz := createHelmPackage(t, c.ChartName, c.Version)
		_ = cp.Copy(tgz, filepath.Join(tmpDir, filepath.Base(tgz)))
	}

	fs := http.FileServer(http.FS(os.DirFS(tmpDir)))
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if password != "" {
			u, p, ok := r.BasicAuth()
			if !ok || u != username || p != password {
				http.Error(w, "Auth header was incorrect", http.StatusUnauthorized)
				return
			}
		}
		fs.ServeHTTP(w, r)
	}))

	t.Cleanup(s.Close)

	i, err := repo.IndexDirectory(tmpDir, s.URL)
	if err != nil {
		t.Fatal(err)
	}

	i.SortEntries()
	err = i.WriteFile(filepath.Join(tmpDir, "index.yaml"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	return s.URL
}

func CreateHelmOciRepo(t *testing.T, charts []RepoChart, username string, password string) string {
	tmpDir := t.TempDir()
	ociUrl := CreateOciRegistry(t, username, password)

	var out strings.Builder
	settings := cli.New()
	c := uploader.ChartUploader{
		Out:     &out,
		Pushers: pusher.All(settings),
		Options: []pusher.Option{},
	}

	var registryClient *registry2.Client
	if password != "" {
		var err error
		registryClient, err = registry2.NewClient()
		if err != nil {
			t.Fatal(err)
		}
		err = registryClient.Login(ociUrl.Host, registry2.LoginOptBasicAuth(username, password), registry2.LoginOptInsecure(true))
		if err != nil {
			t.Fatal(err)
		}
		c.Options = append(c.Options, pusher.WithRegistryClient(registryClient))
	}

	for _, chart := range charts {
		tgz := createHelmPackage(t, chart.ChartName, chart.Version)
		_ = cp.Copy(tgz, filepath.Join(tmpDir, filepath.Base(tgz)))

		err := c.UploadTo(tgz, ociUrl.String())
		if err != nil {
			t.Fatal(err)
		}
	}

	if registryClient != nil {
		registryClient.Logout(ociUrl.Host)
	}

	return ociUrl.String()
}
