package e2e

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func createHelmOrOciRepo(t *testing.T, charts []test_utils.RepoChart, oci bool, user string, password string) string {
	if oci {
		return test_utils.CreateOciRepo(t, charts, user, password)
	} else {
		return test_utils.CreateHelmRepo(t, charts, user, password)
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

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	user := ""
	password := ""
	if tc.testAuth {
		user = "test-user"
		password = "secret-password"
	}

	repoUrl := createHelmOrOciRepo(t, []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
	}, tc.oci, user, password)

	p.UpdateTarget("test", nil)
	p.AddHelmDeployment("helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)

	if tc.testAuth {
		if tc.credsId != "" {
			p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
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
			assert.FileExists(t, getChartFile(t, p, repoUrl, "test-chart1", "0.1.0"))
		}
	} else {
		p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
			_ = o.SetNestedField(true, "helmChart", "skipPrePull")
			return nil
		}, "")
	}

	args := []string{"deploy", "--yes", "-t", "test"}
	args = append(args, tc.extraArgs...)
	_, stderr, err := p.Kluctl(args...)
	pullMessage := "Pulling Helm Chart test-chart1 with version 0.1.0"
	if prePull {
		assert.NotContains(t, stderr, pullMessage)
	} else {
		assert.Contains(t, stderr, pullMessage)
	}
	if tc.expectedError != "" {
		assert.Error(t, err)
		assert.Contains(t, stderr, tc.expectedError)
	} else {
		assert.NoError(t, err)
		assertConfigMapExists(t, k, p.TestSlug(), "test-helm1-test-chart1")
	}
}

func TestHelmPull(t *testing.T) {
	tests := []testCase{
		{name: "helm-no-creds"},
		{name: "helm-creds-missing", oci: false, testAuth: true, credsId: "test-creds",
			expectedError: "401 Unauthorized"},
		{name: "helm-creds-invalid", oci: false, testAuth: true, credsId: "test-creds",
			extraArgs:     []string{"--helm-username=test-creds:test-user", "--helm-password=test-creds:invalid"},
			expectedError: "401 Unauthorized"},
		{name: "helm-creds-valid", oci: false, testAuth: true, credsId: "test-creds",
			extraArgs: []string{"--helm-username=test-creds:test-user", "--helm-password=test-creds:secret-password"}},
		{name: "oci", oci: true},
		{name: "oci-creds-fail", oci: true, testAuth: true, credsId: "test-creds",
			extraArgs:     []string{"--helm-username=test-creds:test-user", "--helm-password=test-creds:secret-password"},
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

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	repoUrl := createHelmOrOciRepo(t, []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart1", Version: "0.2.0"},
	}, oci, "", "")

	p.UpdateTarget("test", nil)
	p.AddHelmDeployment("helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)

	p.KluctlMust("helm-pull")
	assert.FileExists(t, getChartFile(t, p, repoUrl, "test-chart1", "0.1.0"))
	p.KluctlMust("deploy", "--yes", "-t", "test")
	cm := assertConfigMapExists(t, k, p.TestSlug(), "test-helm1-test-chart1")
	v, _, _ := cm.GetNestedString("data", "version")
	assert.Equal(t, "0.1.0", v)

	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("0.2.0", "helmChart", "chartVersion")
		return nil
	}, "")

	p.KluctlMust("helm-pull")
	assert.NoFileExists(t, getChartFile(t, p, repoUrl, "test-chart1", "0.1.0"))
	assert.FileExists(t, getChartFile(t, p, repoUrl, "test-chart1", "0.2.0"))
	p.KluctlMust("deploy", "--yes", "-t", "test")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "test-helm1-test-chart1")
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

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	repoUrl := createHelmOrOciRepo(t, []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart1", Version: "0.2.0"},
		{ChartName: "test-chart2", Version: "0.1.0"},
		{ChartName: "test-chart2", Version: "0.3.0"},
	}, oci, "", "")

	p.UpdateTarget("test", nil)
	p.AddHelmDeployment("helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)
	p.AddHelmDeployment("helm2", repoUrl, "test-chart2", "0.1.0", "test-helm2", p.TestSlug(), nil)
	p.AddHelmDeployment("helm3", repoUrl, "test-chart1", "0.1.0", "test-helm3", p.TestSlug(), nil)

	p.UpdateYaml("helm3/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipUpdate")
		return nil
	}, "")

	p.KluctlMust("helm-pull")
	assert.FileExists(t, getChartFile(t, p, repoUrl, "test-chart1", "0.1.0"))
	assert.FileExists(t, getChartFile(t, p, repoUrl, "test-chart2", "0.1.0"))

	if commit {
		wt, _ := p.GetGitRepo().Worktree()
		_, _ = wt.Add(".helm-charts")
		_, _ = wt.Commit(".helm-charts", &git.CommitOptions{})
	}

	args := []string{"helm-update"}
	if upgrade {
		args = append(args, "--upgrade")
	}
	if commit {
		args = append(args, "--commit")
	}

	_, stderr := p.KluctlMust(args...)
	assert.Contains(t, stderr, "helm1: Chart test-chart1 has new version 0.2.0 available")
	assert.Contains(t, stderr, "helm2: Chart test-chart2 has new version 0.3.0 available")
	assert.Contains(t, stderr, "helm3: Skipped update to version 0.2.0")

	if upgrade {
		assert.Contains(t, stderr, "Upgrading Chart test-chart1 from version 0.1.0 to 0.2.0")
		assert.Contains(t, stderr, "Upgrading Chart test-chart2 from version 0.1.0 to 0.3.0")
	}
	if commit {
		assert.Contains(t, stderr, "Committed helm chart test-chart1 with version 0.2.0")
		assert.Contains(t, stderr, "Committed helm chart test-chart2 with version 0.3.0")
	}

	pulledVersions1 := listChartVersions(t, p, repoUrl, "test-chart1")
	pulledVersions2 := listChartVersions(t, p, repoUrl, "test-chart2")

	if upgrade {
		assert.Equal(t, []string{"0.1.0", "0.2.0"}, pulledVersions1)
		assert.Equal(t, []string{"0.3.0"}, pulledVersions2)
	} else {
		assert.Equal(t, []string{"0.1.0"}, pulledVersions1)
		assert.Equal(t, []string{"0.1.0"}, pulledVersions2)
	}

	if commit {
		r := p.GetGitRepo()

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

		assert.Equal(t, "Updated helm chart test-chart1 from version 0.1.0 to version 0.2.0", commitList[0].Message)
		assert.Equal(t, "Updated helm chart test-chart2 from version 0.1.0 to version 0.3.0", commitList[1].Message)
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

func testHelmUpdateConstraints(t *testing.T, oci bool) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	repoUrl := createHelmOrOciRepo(t, []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart1", Version: "0.1.1"},
		{ChartName: "test-chart1", Version: "0.2.0"},
		{ChartName: "test-chart1", Version: "1.1.0"},
		{ChartName: "test-chart1", Version: "1.1.1"},
		{ChartName: "test-chart1", Version: "1.2.1"},
	}, oci, "", "")

	p.UpdateTarget("test", nil)
	p.AddHelmDeployment("helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)
	p.AddHelmDeployment("helm2", repoUrl, "test-chart1", "0.1.0", "test-helm2", p.TestSlug(), nil)
	p.AddHelmDeployment("helm3", repoUrl, "test-chart1", "0.1.0", "test-helm3", p.TestSlug(), nil)

	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("~0.1.0", "helmChart", "updateConstraints")
		return nil
	}, "")
	p.UpdateYaml("helm2/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("~0.2.0", "helmChart", "updateConstraints")
		return nil
	}, "")
	p.UpdateYaml("helm3/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("~1.2.0", "helmChart", "updateConstraints")
		return nil
	}, "")

	args := []string{"helm-update", "--upgrade"}

	_, stderr := p.KluctlMust(args...)
	assert.Contains(t, stderr, "helm1: Chart test-chart1 has new version 0.1.1 available")
	assert.Contains(t, stderr, "helm2: Chart test-chart1 has new version 0.2.0 available")
	assert.Contains(t, stderr, "helm3: Chart test-chart1 has new version 1.2.1 available")

	c1 := p.GetYaml("helm1/helm-chart.yaml")
	c2 := p.GetYaml("helm2/helm-chart.yaml")
	c3 := p.GetYaml("helm3/helm-chart.yaml")

	v1, _, _ := c1.GetNestedString("helmChart", "chartVersion")
	v2, _, _ := c2.GetNestedString("helmChart", "chartVersion")
	v3, _, _ := c3.GetNestedString("helmChart", "chartVersion")
	assert.Equal(t, "0.1.1", v1)
	assert.Equal(t, "0.2.0", v2)
	assert.Equal(t, "1.2.1", v3)
}

func TestHelmUpdateConstraints(t *testing.T) {
	testHelmUpdateConstraints(t, false)
}

func TestHelmUpdateConstraintsOci(t *testing.T) {
	testHelmUpdateConstraints(t, true)
}

func TestHelmValues(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	repoUrl := test_utils.CreateHelmRepo(t, []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart2", Version: "0.1.0"},
	}, "", "")

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

	p.UpdateTarget("test", nil)
	p.AddHelmDeployment("helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), values1)
	p.AddHelmDeployment("helm2", repoUrl, "test-chart2", "0.1.0", "test-helm2", p.TestSlug(), values2)
	p.AddHelmDeployment("helm3", repoUrl, "test-chart1", "0.1.0", "test-helm3", p.TestSlug(), values3)

	p.KluctlMust("helm-pull")
	p.KluctlMust("deploy", "--yes", "-t", "test", "-aa=a", "-ab=b")

	cm1 := assertConfigMapExists(t, k, p.TestSlug(), "test-helm1-test-chart1")
	cm2 := assertConfigMapExists(t, k, p.TestSlug(), "test-helm2-test-chart2")
	cm3 := assertConfigMapExists(t, k, p.TestSlug(), "test-helm3-test-chart1")

	assert.Equal(t, map[string]any{
		"a":           "x1",
		"b":           "y1",
		"version":     "0.1.0",
		"kubeVersion": k.ServerVersion.String(),
	}, cm1.Object["data"])
	assert.Equal(t, map[string]any{
		"a":           "x2",
		"b":           "y2",
		"version":     "0.1.0",
		"kubeVersion": k.ServerVersion.String(),
	}, cm2.Object["data"])
	assert.Equal(t, map[string]any{
		"a":           "a",
		"b":           "b",
		"version":     "0.1.0",
		"kubeVersion": k.ServerVersion.String(),
	}, cm3.Object["data"])
}

func TestHelmTemplateChartYaml(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())
	createNamespace(t, k, p.TestSlug()+"-a")
	createNamespace(t, k, p.TestSlug()+"-b")

	repoUrl := test_utils.CreateHelmRepo(t, []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart2", Version: "0.1.0"},
	}, "", "")

	p.UpdateTarget("test", nil)
	p.AddHelmDeployment("helm1", repoUrl, "test-chart1", "0.1.0", "test-helm-{{ args.a }}", p.TestSlug(), nil)
	p.AddHelmDeployment("helm2", repoUrl, "test-chart2", "0.1.0", "test-helm-{{ args.b }}", p.TestSlug(), nil)
	p.AddHelmDeployment("helm3", repoUrl, "test-chart1", "0.1.0", "test-helm-ns", p.TestSlug()+"-{{ args.a }}", nil)
	p.AddHelmDeployment("helm4", repoUrl, "test-chart1", "0.1.0", "test-helm-ns", p.TestSlug()+"-{{ args.b }}", nil)

	p.KluctlMust("helm-pull")
	p.KluctlMust("deploy", "--yes", "-t", "test", "-aa=a", "-ab=b")

	assertConfigMapExists(t, k, p.TestSlug(), "test-helm-a-test-chart1")
	assertConfigMapExists(t, k, p.TestSlug(), "test-helm-b-test-chart2")
	assertConfigMapExists(t, k, p.TestSlug()+"-a", "test-helm-ns-test-chart1")
	assertConfigMapExists(t, k, p.TestSlug()+"-b", "test-helm-ns-test-chart1")
}

func TestHelmRenderOfflineKubernetes(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	repoUrl := test_utils.CreateHelmRepo(t, []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
	}, "", "")

	p.UpdateTarget("test", nil)
	p.AddHelmDeployment("helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)
	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipPrePull")
		return nil
	}, "")

	stdout, _ := p.KluctlMust("render", "--print-all", "--offline-kubernetes", "-t", "test")
	cm1 := uo.FromStringMust(stdout)

	assert.Equal(t, map[string]any{
		"a":           "v1",
		"b":           "v2",
		"version":     "0.1.0",
		"kubeVersion": "v1.20.0",
	}, cm1.Object["data"])

	stdout, _ = p.KluctlMust("render", "--print-all", "--offline-kubernetes", "--kubernetes-version", "1.22.1", "-t", "test")
	cm1 = uo.FromStringMust(stdout)

	assert.Equal(t, map[string]any{
		"a":           "v1",
		"b":           "v2",
		"version":     "0.1.0",
		"kubeVersion": "v1.22.1",
	}, cm1.Object["data"])
}

func TestHelmLocalChart(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)
	p.AddHelmDeployment("helm1", "../test-chart1", "", "", "test-helm-1", p.TestSlug(), nil)
	p.AddHelmDeployment("helm2", "test-chart2", "", "", "test-helm-2", p.TestSlug(), nil)

	test_utils.CreateHelmDir(t, "test-chart1", "0.1.0", filepath.Join(p.LocalProjectDir(), "test-chart1"))
	test_utils.CreateHelmDir(t, "test-chart2", "0.1.0", filepath.Join(p.LocalProjectDir(), "helm2/test-chart2"))

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "test-helm-1-test-chart1")
	assertConfigMapExists(t, k, p.TestSlug(), "test-helm-2-test-chart2")

	_, stderr := p.KluctlMust("helm-pull")
	assert.NotContains(t, stderr, "test-chart1")
	assert.NotContains(t, stderr, "test-chart2")
}

func TestHelmSkipPrePull(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	repoUrl := createHelmOrOciRepo(t, []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart1", Version: "0.1.1"},
		{ChartName: "test-chart1", Version: "0.2.0"},
	}, false, "", "")
	u, _ := url.Parse(repoUrl)

	p.UpdateTarget("test", nil)
	p.AddHelmDeployment("helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)
	p.AddHelmDeployment("helm2", repoUrl, "test-chart1", "0.1.1", "test-helm2", p.TestSlug(), nil)

	p.UpdateYaml("helm2/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipPrePull")
		return nil
	}, "")

	args := []string{"helm-pull"}

	_, stderr := p.KluctlMust(args...)
	assert.Contains(t, stderr, "Pulling Chart with version 0.1.0")
	assert.NotContains(t, stderr, "version 0.1.1")
	assert.DirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.0", u.Port())))
	assert.NoDirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.1", u.Port())))

	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipPrePull")
		return nil
	}, "")
	_, stderr = p.KluctlMust(args...)
	assert.Contains(t, stderr, "Removing unused Chart with version 0.1.0")
	assert.NotContains(t, stderr, "version 0.1.1")
	assert.NoDirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.0", u.Port())))
	assert.NoDirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.1", u.Port())))

	p.UpdateYaml("helm2/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(false, "helmChart", "skipPrePull")
		return nil
	}, "")
	_, stderr = p.KluctlMust(args...)
	assert.Contains(t, stderr, "test-chart1: Pulling Chart with version 0.1.1")
	assert.NotContains(t, stderr, "version 0.1.0")
	assert.NoDirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.0", u.Port())))
	assert.DirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.1", u.Port())))

	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(false, "helmChart", "skipPrePull")
		return nil
	}, "")
	_, stderr = p.KluctlMust(args...)
	assert.Contains(t, stderr, "Pulling Chart with version 0.1.0")
	assert.Contains(t, stderr, "Pulling Chart with version 0.1.1")
	assert.DirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.0", u.Port())))
	assert.DirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.1", u.Port())))

	// not try to update+pull
	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(true, "helmChart", "skipPrePull")
		return nil
	}, "")
	_, stderr = p.KluctlMust(args...)
	p.GitServer().CommitFiles("kluctl-project", []string{".helm-charts"}, false, ".helm-charts")
	args = []string{
		"helm-update",
		"--upgrade",
		"--commit",
	}
	_, stderr = p.KluctlMust(args...)
	assert.NotContains(t, stderr, "Pulling Chart with version 0.1.0")
	assert.NotContains(t, stderr, "Pulling Chart with version 0.1.1")
	assert.Contains(t, stderr, "Pulling Chart with version 0.2.0")
	assert.NoDirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.0", u.Port())))
	assert.NoDirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.1.1", u.Port())))
	assert.DirExists(t, filepath.Join(p.LocalProjectDir(), fmt.Sprintf(".helm-charts/http_%s_127.0.0.1/test-chart1/0.2.0", u.Port())))
}

func getChartDir(t *testing.T, p *test_utils.TestProject, url2 string, chartName string, chartVersion string) string {
	u, err := url.Parse(url2)
	if err != nil {
		t.Fatal(err)
	}
	var dir string
	if u.Scheme == "oci" {
		dir = filepath.Join(p.LocalProjectDir(), ".helm-charts", fmt.Sprintf("%s_%s", u.Scheme, u.Hostname()), chartName)
	} else {
		dir = filepath.Join(p.LocalProjectDir(), ".helm-charts", fmt.Sprintf("%s_%s_%s", u.Scheme, u.Port(), u.Hostname()), chartName)
	}
	if chartVersion != "" {
		dir = filepath.Join(dir, chartVersion)
	}
	return dir
}

func getChartFile(t *testing.T, p *test_utils.TestProject, url2 string, chartName string, chartVersion string) string {
	return filepath.Join(getChartDir(t, p, url2, chartName, chartVersion), "Chart.yaml")
}

func listChartVersions(t *testing.T, p *test_utils.TestProject, url2 string, chartName string) []string {
	des, err := os.ReadDir(getChartDir(t, p, url2, chartName, ""))
	assert.NoError(t, err)

	var versions []string
	for _, de := range des {
		versions = append(versions, de.Name())
	}
	sort.Strings(versions)
	return versions
}
