package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/lib/yaml"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"strings"
	"testing"
)

type preparedSourceOverrideTest struct {
	p   *test_project.TestProject
	ip1 *test_project.TestProject
	ip2 *test_project.TestProject

	repo     *test_utils.TestHelmRepo
	repoUrl1 string
	repoUrl2 string

	overrideGroupDir string
	override1        string
	override2        string
}

func prepareLocalSourceOverrideTest(t *testing.T, k *test_utils.EnvTestCluster, gs *test_utils.TestGitServer, oci bool) preparedSourceOverrideTest {
	p := test_project.NewTestProject(t, test_project.WithGitServer(gs))
	ip1 := prepareIncludeProject(t, "include1", "", gs)
	ip2 := prepareIncludeProject(t, "include2", "subDir", gs)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {})

	repo := &test_utils.TestHelmRepo{
		Oci: true,
	}
	if oci {
		repo.Start(t)
	}

	repo1 := repo.URL.String() + "/org1/include1"
	repo2 := repo.URL.String() + "/org1/include2"

	if oci {
		ip1.KluctlMust(t, "oci", "push", "--url", repo1)
		ip2.KluctlMust(t, "oci", "push", "--url", repo2, "--project-dir", ip2.LocalWorkDir())
	}

	if oci {
		p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
			"oci": map[string]any{
				"url": repo1,
			},
		}))
		p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
			"oci": map[string]any{
				"url":    repo2,
				"subDir": "subDir",
			},
		}))
	} else {
		p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
			"git": map[string]any{
				"url": ip1.GitUrl(),
			},
		}))
		p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
			"git": map[string]any{
				"url":    ip2.GitUrl(),
				"subDir": "subDir",
			},
		}))
	}

	overrideGroupDir := t.TempDir()

	override1 := ip1.CopyProjectSourceTo(filepath.Join(overrideGroupDir, "include1"))
	override2 := ip2.CopyProjectSourceTo(filepath.Join(overrideGroupDir, "include2"))

	cm, err := uo.FromFile(filepath.Join(override1, "cm", "configmap-include1-cm.yml"))
	assert.NoError(t, err)
	_ = cm.SetNestedField("o1", "data", "a")
	_ = yaml.WriteYamlFile(filepath.Join(override1, "cm", "configmap-include1-cm.yml"), cm)

	cm, err = uo.FromFile(filepath.Join(override2, "subDir", "cm", "configmap-include2-cm.yml"))
	assert.NoError(t, err)
	_ = cm.SetNestedField("o2", "data", "a")
	_ = yaml.WriteYamlFile(filepath.Join(override2, "subDir", "cm", "configmap-include2-cm.yml"), cm)

	return preparedSourceOverrideTest{
		p:                p,
		ip1:              ip1,
		ip2:              ip2,
		repo:             repo,
		repoUrl1:         repo1,
		repoUrl2:         repo2,
		overrideGroupDir: overrideGroupDir,
		override1:        override1,
		override2:        override2,
	}
}

func TestLocalGitOverride(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	pt := prepareLocalSourceOverrideTest(t, k, nil, false)

	u1, _ := types.ParseGitUrl(pt.ip1.GitUrl())
	u2, _ := types.ParseGitUrl(pt.ip2.GitUrl())
	k1 := u1.RepoKey().String()
	k2 := u2.RepoKey().String()

	pt.p.KluctlMust(t, "deploy", "--yes", "-t", "test",
		"--local-git-override", fmt.Sprintf("%s=%s", k1, pt.override1),
		"--local-git-override", fmt.Sprintf("%s=%s", k2, pt.override2),
	)
	cm := assertConfigMapExists(t, k, pt.p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "o1", "data", "a")
	cm = assertConfigMapExists(t, k, pt.p.TestSlug(), "include2-cm")
	assertNestedFieldEquals(t, cm, "o2", "data", "a")
}

func TestGitGroup(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	gs := test_utils.NewTestGitServer(t)
	pt := prepareLocalSourceOverrideTest(t, k, gs, false)

	u1, _ := types.ParseGitUrl(pt.p.GitServer().GitUrl() + "/repos")
	k1 := u1.RepoKey().String()

	pt.p.KluctlMust(t, "deploy", "--yes", "-t", "test",
		"--local-git-group-override", fmt.Sprintf("%s=%s", k1, pt.overrideGroupDir),
	)
	cm := assertConfigMapExists(t, k, pt.p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "o1", "data", "a")
	cm = assertConfigMapExists(t, k, pt.p.TestSlug(), "include2-cm")
	assertNestedFieldEquals(t, cm, "o2", "data", "a")
}

func TestLocalOciOverride(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	pt := prepareLocalSourceOverrideTest(t, k, nil, true)

	k1 := strings.TrimPrefix(pt.repoUrl1, "oci://")
	k2 := strings.TrimPrefix(pt.repoUrl2, "oci://")

	pt.p.KluctlMust(t, "deploy", "--yes", "-t", "test",
		"--local-oci-override", fmt.Sprintf("%s=%s", k1, pt.override1),
		"--local-oci-override", fmt.Sprintf("%s=%s", k2, pt.override2),
	)
	cm := assertConfigMapExists(t, k, pt.p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "o1", "data", "a")
	cm = assertConfigMapExists(t, k, pt.p.TestSlug(), "include2-cm")
	assertNestedFieldEquals(t, cm, "o2", "data", "a")
}

func TestLocalOciGroupOverride(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	pt := prepareLocalSourceOverrideTest(t, k, nil, true)

	k1 := strings.TrimPrefix(pt.repo.URL.String(), "oci://") + "/org1"

	pt.p.KluctlMust(t, "deploy", "--yes", "-t", "test",
		"--local-oci-group-override", fmt.Sprintf("%s=%s", k1, pt.overrideGroupDir),
	)
	cm := assertConfigMapExists(t, k, pt.p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "o1", "data", "a")
	cm = assertConfigMapExists(t, k, pt.p.TestSlug(), "include2-cm")
	assertNestedFieldEquals(t, cm, "o2", "data", "a")
}
