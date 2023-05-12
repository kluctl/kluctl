package e2e

import (
	"fmt"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	git2 "github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func prepareIncludeProject(t *testing.T, prefix string, subDir string, gitServer *git2.TestGitServer) *test_utils.TestProject {
	p := test_utils.NewTestProject(t,
		test_utils.WithGitSubDir(subDir),
		test_utils.WithGitServer(gitServer),
		test_utils.WithRepoName(fmt.Sprintf("repos/%s", prefix)),
	)
	addConfigMapDeployment(p, "cm", map[string]string{"a": "v"}, resourceOpts{
		name:      fmt.Sprintf("%s-cm", prefix),
		namespace: p.TestSlug(),
	})
	return p
}

func prepareGitIncludeTest(t *testing.T, gitServer *git2.TestGitServer) (*test_utils.TestProject, *test_utils.TestProject, *test_utils.TestProject) {
	k := defaultCluster1

	p := test_utils.NewTestProject(t, test_utils.WithGitServer(gitServer))
	ip1 := prepareIncludeProject(t, "include1", "", gitServer)
	ip2 := prepareIncludeProject(t, "include2", "subDir", gitServer)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {})

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

	return p, ip1, ip2
}

func TestGitInclude(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p, _, _ := prepareGitIncludeTest(t, nil)

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertConfigMapExists(t, k, p.TestSlug(), "include2-cm")
}

func TestLocalGitOverride(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	p, ip1, ip2 := prepareGitIncludeTest(t, nil)

	override1 := t.TempDir()
	err := cp.Copy(ip1.LocalRepoDir(), override1)
	assert.NoError(t, err)

	override2 := t.TempDir()
	err = cp.Copy(ip2.LocalRepoDir(), override2)
	assert.NoError(t, err)

	cm, err := uo.FromFile(filepath.Join(override1, "cm", "configmap-include1-cm.yml"))
	assert.NoError(t, err)
	_ = cm.SetNestedField("o1", "data", "a")
	_ = yaml.WriteYamlFile(filepath.Join(override1, "cm", "configmap-include1-cm.yml"), cm)

	cm, err = uo.FromFile(filepath.Join(override2, "subDir", "cm", "configmap-include2-cm.yml"))
	assert.NoError(t, err)
	_ = cm.SetNestedField("o2", "data", "a")
	_ = yaml.WriteYamlFile(filepath.Join(override2, "subDir", "cm", "configmap-include2-cm.yml"), cm)

	u1, _ := types.ParseGitUrl(ip1.GitUrl())
	u2, _ := types.ParseGitUrl(ip2.GitUrl())
	k1 := u1.NormalizedRepoKey()
	k2 := u2.NormalizedRepoKey()

	p.KluctlMust("deploy", "--yes", "-t", "test",
		"--local-git-override", fmt.Sprintf("%s=%s", k1, override1),
		"--local-git-override", fmt.Sprintf("%s=%s", k2, override2),
	)
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "o1", "data", "a")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include2-cm")
	assertNestedFieldEquals(t, cm, "o2", "data", "a")
}

func TestLocalGitGroupOverride(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	p, ip1, ip2 := prepareGitIncludeTest(t, git2.NewTestGitServer(t))

	overrideGroupDir := t.TempDir()

	override1 := filepath.Join(overrideGroupDir, "include1")
	err := cp.Copy(ip1.LocalRepoDir(), override1)
	assert.NoError(t, err)

	override2 := filepath.Join(overrideGroupDir, "include2")
	err = cp.Copy(ip2.LocalRepoDir(), override2)
	assert.NoError(t, err)

	cm, err := uo.FromFile(filepath.Join(override1, "cm", "configmap-include1-cm.yml"))
	assert.NoError(t, err)
	_ = cm.SetNestedField("o1", "data", "a")
	_ = yaml.WriteYamlFile(filepath.Join(override1, "cm", "configmap-include1-cm.yml"), cm)

	cm, err = uo.FromFile(filepath.Join(override2, "subDir", "cm", "configmap-include2-cm.yml"))
	assert.NoError(t, err)
	_ = cm.SetNestedField("o2", "data", "a")
	_ = yaml.WriteYamlFile(filepath.Join(override2, "subDir", "cm", "configmap-include2-cm.yml"), cm)

	u1, _ := types.ParseGitUrl(p.GitServer().GitUrl() + "/repos")
	k1 := u1.NormalizedRepoKey()

	p.KluctlMust("deploy", "--yes", "-t", "test",
		"--local-git-group-override", fmt.Sprintf("%s=%s", k1, overrideGroupDir),
	)
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "o1", "data", "a")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include2-cm")
	assertNestedFieldEquals(t, cm, "o2", "data", "a")
}
