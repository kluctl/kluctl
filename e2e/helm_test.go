package e2e

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/google/go-containerregistry/pkg/registry"
	test_resources "github.com/kluctl/kluctl/v2/e2e/test-helm-chart"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/pusher"
	registry2 "helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/uploader"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func createHelmPackage(t *testing.T, name string, version string) string {
	tmpDir := t.TempDir()
	err := utils.FsCopyDir(test_resources.HelmChartFS, ".", tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	c, err := uo.FromFile(filepath.Join(tmpDir, "Chart.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	_ = c.SetNestedField(name, "name")
	_ = c.SetNestedField(version, "version")

	err = yaml.WriteYamlFile(filepath.Join(tmpDir, "Chart.yaml"), c)
	if err != nil {
		t.Fatal(err)
	}

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

type repoChart struct {
	chartName string
	version   string
}

func createHelmRepo(t *testing.T, charts []repoChart, password string) string {
	tmpDir := t.TempDir()

	for _, c := range charts {
		tgz := createHelmPackage(t, c.chartName, c.version)
		_ = utils.CopyFile(tgz, filepath.Join(tmpDir, filepath.Base(tgz)))
	}

	fs := http.FileServer(http.FS(os.DirFS(tmpDir)))
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if password != "" {
			_, p, ok := r.BasicAuth()
			if !ok || p != password {
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

func createOciRepo(t *testing.T, charts []repoChart, password string) string {
	tmpDir := t.TempDir()

	ociRegistry := registry.New()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if password != "" {
			_, p, ok := r.BasicAuth()
			if !ok {
				w.Header().Add("WWW-Authenticate", "Basic")
				http.Error(w, "Auth header was incorrect", http.StatusUnauthorized)
				return
			}
			if !ok || p != password {
				http.Error(w, "Auth header was incorrect", http.StatusUnauthorized)
				return
			}
		}
		ociRegistry.ServeHTTP(w, r)
	}))

	t.Cleanup(s.Close)

	ociUrl := strings.ReplaceAll(s.URL, "http://", "oci://")
	ociUrl2, _ := url.Parse(ociUrl)

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
		err = registryClient.Login(ociUrl2.Host, registry2.LoginOptBasicAuth("test-user", password), registry2.LoginOptInsecure(true))
		if err != nil {
			t.Fatal(err)
		}
		c.Options = append(c.Options, pusher.WithRegistryClient(registryClient))
	}

	for _, chart := range charts {
		tgz := createHelmPackage(t, chart.chartName, chart.version)
		_ = utils.CopyFile(tgz, filepath.Join(tmpDir, filepath.Base(tgz)))

		err := c.UploadTo(tgz, ociUrl)
		if err != nil {
			t.Fatal(err)
		}
	}

	if registryClient != nil {
		registryClient.Logout(ociUrl2.Host)
	}

	return ociUrl
}

func createHelmOrOciRepo(t *testing.T, charts []repoChart, oci bool, password string) string {
	if oci {
		return createOciRepo(t, charts, password)
	} else {
		return createHelmRepo(t, charts, password)
	}
}

func addHelmDeployment(p *testProject, dir string, repoUrl string, chartName, version string, releaseName string, namespace string, values map[string]any) {
	if registry2.IsOCI(repoUrl) {
		repoUrl += "/" + chartName
		chartName = ""
	}

	p.addKustomizeDeployment(dir, []kustomizeResource{
		{name: "helm-rendered.yaml"},
	}, nil)

	p.updateYaml(filepath.Join(dir, "helm-chart.yaml"), func(o *uo.UnstructuredObject) error {
		*o = *uo.FromMap(map[string]interface{}{
			"helmChart": map[string]any{
				"repo":         repoUrl,
				"chartVersion": version,
				"releaseName":  releaseName,
				"namespace":    namespace,
			},
		})
		if chartName != "" {
			_ = o.SetNestedField(chartName, "helmChart", "chartName")
		}
		return nil
	}, "")

	if values != nil {
		p.updateYaml(filepath.Join(dir, "helm-values.yaml"), func(o *uo.UnstructuredObject) error {
			*o = *uo.FromMap(values)
			return nil
		}, "")
	}
}

type testCase struct {
	name          string
	oci           bool
	testAuth      bool
	credsId       string
	extraArgs     []string
	expectedError string
}

func testHelmPull(t *testing.T, tc testCase, prePull bool) {
	t.Parallel()

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k)

	createNamespace(t, k, p.testSlug())

	password := ""
	if tc.testAuth {
		password = "secret-password"
	}

	repoUrl := createHelmOrOciRepo(p.t, []repoChart{
		{chartName: "test-chart1", version: "0.1.0"},
	}, tc.oci, password)

	p.updateTarget("test", nil)
	addHelmDeployment(p, "helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.testSlug(), nil)

	if tc.testAuth {
		if tc.credsId != "" {
			p.updateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
				_ = o.SetNestedField(tc.credsId, "helmChart", "credentialsId")
				return nil
			}, "")
		}
	}

	if prePull {
		args := []string{"helm-pull"}
		args = append(args, tc.extraArgs...)

		_, stderr, err := p.Kluctl(args...)
		if tc.expectedError != "" {
			assert.Error(t, err)
			assert.Contains(t, stderr, tc.expectedError)
			return
		} else {
			assert.NoError(t, err)
			assert.FileExists(t, filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), "helm1/charts/test-chart1/Chart.yaml"))
		}
	}

	args := []string{"deploy", "--yes", "-t", "test"}
	args = append(args, tc.extraArgs...)
	_, stderr, err := p.Kluctl(args...)
	prePullWarning := "Warning, need to pull Helm Chart test-chart1 with version 0.1.0."
	if prePull {
		assert.NotContains(t, stderr, prePullWarning)
	} else {
		assert.Contains(t, stderr, prePullWarning)
	}
	if tc.expectedError != "" {
		assert.Error(t, err)
		assert.Contains(t, stderr, tc.expectedError)
	} else {
		assert.NoError(t, err)
		assertConfigMapExists(t, k, p.testSlug(), "test-helm1-test-chart1")
	}
}

func TestHelmPull(t *testing.T) {
	tests := []testCase{
		{name: "helm-no-creds"},
		{name: "helm-creds-missing", oci: false, testAuth: true, credsId: "test-creds",
			expectedError: "no credentials provided for Chart test-chart1"},
		{name: "helm-creds-invalid", oci: false, testAuth: true, credsId: "test-creds",
			extraArgs:     []string{"--helm-username=test-creds:user", "--helm-password=test-creds:invalid"},
			expectedError: "401 Unauthorized"},
		{name: "helm-creds-valid", oci: false, testAuth: true, credsId: "test-creds",
			extraArgs: []string{"--helm-username=test-creds:user", "--helm-password=test-creds:secret-password"}},
		{name: "oci", oci: true},
		{name: "oci-creds-fail", oci: true, testAuth: true, credsId: "test-creds",
			extraArgs:     []string{"--helm-username=test-creds:user", "--helm-password=test-creds:secret-password"},
			expectedError: "OCI charts can currently only be authenticated via registry login and not via cli arguments"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			testHelmPull(t, tc, false)
		})
		t.Run(tc.name+"-prepull", func(t *testing.T) {
			testHelmPull(t, tc, true)
		})
	}
}

func testHelmManualUpgrade(t *testing.T, oci bool) {
	t.Parallel()

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k)

	createNamespace(t, k, p.testSlug())

	repoUrl := createHelmOrOciRepo(p.t, []repoChart{
		{chartName: "test-chart1", version: "0.1.0"},
		{chartName: "test-chart1", version: "0.2.0"},
	}, oci, "")

	p.updateTarget("test", nil)
	addHelmDeployment(p, "helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.testSlug(), nil)

	p.KluctlMust("helm-pull")
	assert.FileExists(t, filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), "helm1/charts/test-chart1/Chart.yaml"))
	p.KluctlMust("deploy", "--yes", "-t", "test")
	cm := assertConfigMapExists(t, k, p.testSlug(), "test-helm1-test-chart1")
	v, _, _ := cm.GetNestedString("data", "version")
	assert.Equal(t, "0.1.0", v)

	p.updateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("0.2.0", "helmChart", "chartVersion")
		return nil
	}, "")

	p.KluctlMust("helm-pull")
	p.KluctlMust("deploy", "--yes", "-t", "test")
	cm = assertConfigMapExists(t, k, p.testSlug(), "test-helm1-test-chart1")
	v, _, _ = cm.GetNestedString("data", "version")
	assert.Equal(t, "0.2.0", v)
}

func TestHelmManualUpgrade(t *testing.T) {
	testHelmManualUpgrade(t, false)
}

func TestHelmManualUpgradeOci(t *testing.T) {
	testHelmManualUpgrade(t, true)
}

func testHelmUpdate(t *testing.T, oci bool, upgrade bool, commit bool) {
	t.Parallel()

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k)

	createNamespace(t, k, p.testSlug())

	repoUrl := createHelmOrOciRepo(p.t, []repoChart{
		{chartName: "test-chart1", version: "0.1.0"},
		{chartName: "test-chart1", version: "0.2.0"},
		{chartName: "test-chart2", version: "0.1.0"},
		{chartName: "test-chart2", version: "0.3.0"},
	}, oci, "")

	p.updateTarget("test", nil)
	addHelmDeployment(p, "helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.testSlug(), nil)
	addHelmDeployment(p, "helm2", repoUrl, "test-chart2", "0.1.0", "test-helm2", p.testSlug(), nil)
	addHelmDeployment(p, "helm3", repoUrl, "test-chart1", "0.1.0", "test-helm3", p.testSlug(), nil)

	p.updateYaml("helm3/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipUpdate")
		return nil
	}, "")

	p.KluctlMust("helm-pull")
	assert.FileExists(t, filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), "helm1/charts/test-chart1/Chart.yaml"))
	assert.FileExists(t, filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), "helm2/charts/test-chart2/Chart.yaml"))
	assert.FileExists(t, filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), "helm3/charts/test-chart1/Chart.yaml"))

	args := []string{"helm-update"}
	if upgrade {
		args = append(args, "--upgrade")
	}
	if commit {
		args = append(args, "--commit")
	}

	_, stderr := p.KluctlMust(args...)
	assert.Contains(t, stderr, "helm1: Chart has new version 0.2.0 available.")
	assert.Contains(t, stderr, "helm2: Chart has new version 0.3.0 available.")
	assert.Contains(t, stderr, "helm3: Chart has new version 0.2.0 available. Old version is 0.1.0. skipUpdate is set to true.")

	c1, err := uo.FromFile(filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), "helm1/charts/test-chart1/Chart.yaml"))
	assert.NoError(t, err)
	c2, err := uo.FromFile(filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), "helm2/charts/test-chart2/Chart.yaml"))
	assert.NoError(t, err)
	c3, err := uo.FromFile(filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), "helm3/charts/test-chart1/Chart.yaml"))
	assert.NoError(t, err)

	v1, _, _ := c1.GetNestedString("version")
	v2, _, _ := c2.GetNestedString("version")
	v3, _, _ := c3.GetNestedString("version")
	if upgrade {
		assert.Equal(t, "0.2.0", v1)
		assert.Equal(t, "0.3.0", v2)
		assert.Equal(t, "0.1.0", v3)
	} else {
		assert.Equal(t, "0.1.0", v1)
		assert.Equal(t, "0.1.0", v2)
		assert.Equal(t, "0.1.0", v3)
	}

	if commit {
		r := p.gitServer.GetGitRepo(p.getKluctlProjectRepo())

		commits, err := r.Log(&git.LogOptions{})
		assert.NoError(t, err)
		var commitList []object.Commit
		err = commits.ForEach(func(commit *object.Commit) error {
			commitList = append(commitList, *commit)
			return nil
		})
		assert.NoError(t, err)

		commitList = commitList[0:2]
		sort.Slice(commitList, func(i, j int) bool {
			return commitList[i].Message < commitList[j].Message
		})

		assert.Equal(t, "Updated helm chart helm1 from 0.1.0 to 0.2.0", commitList[0].Message)
		assert.Equal(t, "Updated helm chart helm2 from 0.1.0 to 0.3.0", commitList[1].Message)
	}
}

func TestHelmUpdate(t *testing.T) {
	testHelmUpdate(t, false, false, false)
}

func TestHelmUpdateOci(t *testing.T) {
	testHelmUpdate(t, true, false, false)
}

func TestHelmUpdateAndUpgrade(t *testing.T) {
	testHelmUpdate(t, false, true, false)
}

func TestHelmUpdateAndUpgradeOci(t *testing.T) {
	testHelmUpdate(t, true, true, false)
}

func TestHelmUpdateAndUpgradeAndCommit(t *testing.T) {
	testHelmUpdate(t, false, true, true)
}

func TestHelmUpdateAndUpgradeAndCommitOci(t *testing.T) {
	testHelmUpdate(t, true, true, true)
}

func TestHelmValues(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k)

	createNamespace(t, k, p.testSlug())

	repoUrl := createHelmRepo(p.t, []repoChart{
		{chartName: "test-chart1", version: "0.1.0"},
		{chartName: "test-chart2", version: "0.1.0"},
	}, "")

	values1 := map[string]any{
		"data": map[string]any{
			"a": "x1",
			"b": "y1",
		},
	}
	values2 := map[string]any{
		"data": map[string]any{
			"a": "x2",
			"b": "y2",
		},
	}
	values3 := map[string]any{
		"data": map[string]any{
			"a": "{{ args.a }}",
			"b": "{{ args.b }}",
		},
	}

	p.updateTarget("test", nil)
	addHelmDeployment(p, "helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.testSlug(), values1)
	addHelmDeployment(p, "helm2", repoUrl, "test-chart2", "0.1.0", "test-helm2", p.testSlug(), values2)
	addHelmDeployment(p, "helm3", repoUrl, "test-chart1", "0.1.0", "test-helm3", p.testSlug(), values3)

	p.KluctlMust("helm-pull")
	p.KluctlMust("deploy", "--yes", "-t", "test", "-aa=a", "-ab=b")

	cm1 := assertConfigMapExists(t, k, p.testSlug(), "test-helm1-test-chart1")
	cm2 := assertConfigMapExists(t, k, p.testSlug(), "test-helm2-test-chart2")
	cm3 := assertConfigMapExists(t, k, p.testSlug(), "test-helm3-test-chart1")

	assert.Equal(t, map[string]any{
		"a":       "x1",
		"b":       "y1",
		"version": "0.1.0",
	}, cm1.Object["data"])
	assert.Equal(t, map[string]any{
		"a":       "x2",
		"b":       "y2",
		"version": "0.1.0",
	}, cm2.Object["data"])
	assert.Equal(t, map[string]any{
		"a":       "a",
		"b":       "b",
		"version": "0.1.0",
	}, cm3.Object["data"])
}

func TestHelmTemplateChartYaml(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k)

	createNamespace(t, k, p.testSlug())
	createNamespace(t, k, p.testSlug()+"-a")
	createNamespace(t, k, p.testSlug()+"-b")

	repoUrl := createHelmRepo(p.t, []repoChart{
		{chartName: "test-chart1", version: "0.1.0"},
		{chartName: "test-chart2", version: "0.1.0"},
	}, "")

	p.updateTarget("test", nil)
	addHelmDeployment(p, "helm1", repoUrl, "test-chart1", "0.1.0", "test-helm-{{ args.a }}", p.testSlug(), nil)
	addHelmDeployment(p, "helm2", repoUrl, "test-chart2", "0.1.0", "test-helm-{{ args.b }}", p.testSlug(), nil)
	addHelmDeployment(p, "helm3", repoUrl, "test-chart1", "0.1.0", "test-helm-ns", p.testSlug()+"-{{ args.a }}", nil)
	addHelmDeployment(p, "helm4", repoUrl, "test-chart1", "0.1.0", "test-helm-ns", p.testSlug()+"-{{ args.b }}", nil)

	p.KluctlMust("helm-pull")
	p.KluctlMust("deploy", "--yes", "-t", "test", "-aa=a", "-ab=b")

	assertConfigMapExists(t, k, p.testSlug(), "test-helm-a-test-chart1")
	assertConfigMapExists(t, k, p.testSlug(), "test-helm-b-test-chart2")
	assertConfigMapExists(t, k, p.testSlug()+"-a", "test-helm-ns-test-chart1")
	assertConfigMapExists(t, k, p.testSlug()+"-b", "test-helm-ns-test-chart1")
}
