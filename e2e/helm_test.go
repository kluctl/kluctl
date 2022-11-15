package e2e

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	test_resources "github.com/kluctl/kluctl/v2/e2e/test-helm-chart"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/stretchr/testify/assert"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
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

func createHelmRepo(t *testing.T, charts []repoChart) string {
	tmpDir := t.TempDir()

	for _, c := range charts {
		tgz := createHelmPackage(t, c.chartName, c.version)
		_ = utils.CopyFile(tgz, filepath.Join(tmpDir, filepath.Base(tgz)))
	}

	s := httptest.NewServer(http.FileServer(http.FS(os.DirFS(tmpDir))))
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

func addHelmDeployment(p *testProject, dir string, repoUrl string, chartName, version string, releaseName string, namespace string, values map[string]any) {

	p.addKustomizeDeployment(dir, []kustomizeResource{
		{name: "helm-rendered.yaml"},
	}, nil)

	p.updateYaml(filepath.Join(dir, "helm-chart.yaml"), func(o *uo.UnstructuredObject) error {
		*o = *uo.FromMap(map[string]interface{}{
			"helmChart": map[string]any{
				"repo":         repoUrl,
				"chartName":    chartName,
				"chartVersion": version,
				"releaseName":  releaseName,
				"namespace":    namespace,
			},
		})
		return nil
	}, "")

	if values != nil {
		p.updateYaml(filepath.Join(dir, "helm-values.yaml"), func(o *uo.UnstructuredObject) error {
			*o = *uo.FromMap(values)
			return nil
		}, "")
	}
}

func TestHelmNoPrePull(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k)

	createNamespace(t, k, p.testSlug())

	repoUrl := createHelmRepo(p.t, []repoChart{
		{chartName: "test-chart1", version: "0.1.0"},
	})

	p.updateTarget("test", nil)
	addHelmDeployment(p, "helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.testSlug(), nil)
	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.testSlug(), "test-helm1-test-chart1")
}

func TestHelmPrePull(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k)

	createNamespace(t, k, p.testSlug())

	repoUrl := createHelmRepo(p.t, []repoChart{
		{chartName: "test-chart1", version: "0.1.0"},
		{chartName: "test-chart2", version: "0.1.0"},
	})

	p.updateTarget("test", nil)
	addHelmDeployment(p, "helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.testSlug(), nil)

	p.KluctlMust("helm-pull")
	assert.FileExists(t, filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), "helm1/charts/test-chart1/Chart.yaml"))
	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.testSlug(), "test-helm1-test-chart1")

	addHelmDeployment(p, "helm2", repoUrl, "test-chart2", "0.1.0", "test-helm2", p.testSlug(), nil)

	p.KluctlMust("helm-pull")
	assert.FileExists(t, filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), "helm2/charts/test-chart2/Chart.yaml"))
	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.testSlug(), "test-helm2-test-chart2")
}

func TestHelmManualUpgrade(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k)

	createNamespace(t, k, p.testSlug())

	repoUrl := createHelmRepo(p.t, []repoChart{
		{chartName: "test-chart1", version: "0.1.0"},
		{chartName: "test-chart1", version: "0.2.0"},
	})

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

func testHelmUpdate(t *testing.T, upgrade bool, commit bool) {
	t.Parallel()

	k := defaultCluster1

	p := &testProject{}
	p.init(t, k)

	createNamespace(t, k, p.testSlug())

	repoUrl := createHelmRepo(p.t, []repoChart{
		{chartName: "test-chart1", version: "0.1.0"},
		{chartName: "test-chart1", version: "0.2.0"},
		{chartName: "test-chart2", version: "0.1.0"},
		{chartName: "test-chart2", version: "0.3.0"},
	})

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
	testHelmUpdate(t, false, false)
}

func TestHelmUpdateAndUpgrade(t *testing.T) {
	testHelmUpdate(t, true, false)
}

func TestHelmUpdateAndUpgradeAndCommit(t *testing.T) {
	testHelmUpdate(t, true, true)
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
	})

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
	})

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
