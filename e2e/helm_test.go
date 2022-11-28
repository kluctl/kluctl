package e2e

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
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

	p := test_utils.NewTestProject(t, k)

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
			assert.FileExists(t, filepath.Join(p.LocalRepoDir(), "helm1/charts/test-chart1/Chart.yaml"))
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

	p := test_utils.NewTestProject(t, k)

	createNamespace(t, k, p.TestSlug())

	repoUrl := createHelmOrOciRepo(t, []test_utils.RepoChart{
		{ChartName: "test-chart1", Version: "0.1.0"},
		{ChartName: "test-chart1", Version: "0.2.0"},
	}, oci, "", "")

	p.UpdateTarget("test", nil)
	p.AddHelmDeployment("helm1", repoUrl, "test-chart1", "0.1.0", "test-helm1", p.TestSlug(), nil)

	p.KluctlMust("helm-pull")
	assert.FileExists(t, filepath.Join(p.LocalRepoDir(), "helm1/charts/test-chart1/Chart.yaml"))
	p.KluctlMust("deploy", "--yes", "-t", "test")
	cm := assertConfigMapExists(t, k, p.TestSlug(), "test-helm1-test-chart1")
	v, _, _ := cm.GetNestedString("data", "version")
	assert.Equal(t, "0.1.0", v)

	p.UpdateYaml("helm1/helm-chart.yaml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("0.2.0", "helmChart", "chartVersion")
		return nil
	}, "")

	p.KluctlMust("helm-pull")
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

	p := test_utils.NewTestProject(t, k)

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
	assert.FileExists(t, filepath.Join(p.LocalRepoDir(), "helm1/charts/test-chart1/Chart.yaml"))
	assert.FileExists(t, filepath.Join(p.LocalRepoDir(), "helm2/charts/test-chart2/Chart.yaml"))
	assert.FileExists(t, filepath.Join(p.LocalRepoDir(), "helm3/charts/test-chart1/Chart.yaml"))

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

	c1, err := uo.FromFile(filepath.Join(p.LocalRepoDir(), "helm1/charts/test-chart1/Chart.yaml"))
	assert.NoError(t, err)
	c2, err := uo.FromFile(filepath.Join(p.LocalRepoDir(), "helm2/charts/test-chart2/Chart.yaml"))
	assert.NoError(t, err)
	c3, err := uo.FromFile(filepath.Join(p.LocalRepoDir(), "helm3/charts/test-chart1/Chart.yaml"))
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

	p := test_utils.NewTestProject(t, k)

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

	p := test_utils.NewTestProject(t, k)

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
