package e2e

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	git2 "github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

func prepareIncludeProject(t *testing.T, prefix string, subDir string, gitServer *git2.TestGitServer) *test_project.TestProject {
	p := test_project.NewTestProject(t,
		test_project.WithGitSubDir(subDir),
		test_project.WithGitServer(gitServer),
		test_project.WithRepoName(fmt.Sprintf("repos/%s", prefix)),
	)
	addConfigMapDeployment(p, "cm", map[string]string{"a": "v"}, resourceOpts{
		name:      fmt.Sprintf("%s-cm", prefix),
		namespace: p.TestSlug(),
	})
	return p
}

func prepareGitIncludeTest(t *testing.T, k *test_utils.EnvTestCluster, mainGs *git2.TestGitServer, gs1 *git2.TestGitServer, gs2 *git2.TestGitServer) (*test_project.TestProject, *test_project.TestProject, *test_project.TestProject) {
	p := test_project.NewTestProject(t, test_project.WithGitServer(mainGs))
	ip1 := prepareIncludeProject(t, "include1", "", gs1)
	ip2 := prepareIncludeProject(t, "include2", "subDir", gs2)

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

	p, _, _ := prepareGitIncludeTest(t, k, nil, nil, nil)

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertConfigMapExists(t, k, p.TestSlug(), "include2-cm")
}

func createBranchAndTag(t *testing.T, p *test_project.TestProject, branchName string, tagName string, tagMessage string, update func()) string {
	err := p.GetGitWorktree().Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("master"),
	})
	assert.NoError(t, err)
	err = p.GetGitWorktree().Checkout(&git.CheckoutOptions{
		Create: true,
		Branch: plumbing.NewBranchReferenceName(branchName),
	})
	assert.NoError(t, err)

	update()

	h, err := p.GetGitRepo().ResolveRevision(plumbing.Revision(branchName))
	assert.NoError(t, err)

	if tagName != "" {
		var to *git.CreateTagOptions
		if tagMessage != "" {
			to = &git.CreateTagOptions{
				Message: tagMessage,
			}
		}
		_, err = p.GetGitRepo().CreateTag(tagName, *h, to)
		assert.NoError(t, err)
	}

	err = p.GetGitWorktree().Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName("master"),
	})
	assert.NoError(t, err)

	return h.String()
}

func TestGitIncludeRef(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_project.NewTestProject(t)
	ip1 := test_project.NewTestProject(t)

	addConfigMapDeployment(p, "cm", map[string]string{"a": "a"}, resourceOpts{
		name:      "parent",
		namespace: p.TestSlug(),
	})

	createBranchAndTag(t, ip1, "branch1", "tag1", "", func() {
		addConfigMapDeployment(ip1, "cm", map[string]string{"a": "branch1"}, resourceOpts{
			name:      "branch1",
			namespace: p.TestSlug(),
		})
	})

	createBranchAndTag(t, ip1, "branch2", "tag2", "", func() {
		addConfigMapDeployment(ip1, "cm", map[string]string{"a": "tag2"}, resourceOpts{
			name:      "tag2",
			namespace: p.TestSlug(),
		})
	})

	createBranchAndTag(t, ip1, "branch3", "tag3", "", func() {
		addConfigMapDeployment(ip1, "cm", map[string]string{"a": "branch3"}, resourceOpts{
			name:      "branch3",
			namespace: p.TestSlug(),
		})
	})

	createBranchAndTag(t, ip1, "branch4", "tag4", "", func() {
		addConfigMapDeployment(ip1, "cm", map[string]string{"a": "tag4"}, resourceOpts{
			name:      "tag4",
			namespace: p.TestSlug(),
		})
	})

	createBranchAndTag(t, ip1, "branch5", "tag5", "tag5", func() {
		addConfigMapDeployment(ip1, "cm", map[string]string{"a": "tag5"}, resourceOpts{
			name:      "tag5",
			namespace: p.TestSlug(),
		})
	})

	commit6 := createBranchAndTag(t, ip1, "branch6", "tag6", "", func() {
		addConfigMapDeployment(ip1, "cm", map[string]string{"a": "tag5"}, resourceOpts{
			name:      "commit6",
			namespace: p.TestSlug(),
		})
	})

	addConfigMapDeployment(ip1, "cm", map[string]string{"a": "HEAD"}, resourceOpts{
		name:      "head",
		namespace: p.TestSlug(),
	})

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {})

	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"git": map[string]any{
			"url": ip1.GitUrl(),
		},
	}))
	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"git": map[string]any{
			"url": ip1.GitUrl(),
			"ref": map[string]any{
				"branch": "branch1",
			},
		},
	}))
	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"git": map[string]any{
			"url": ip1.GitUrl(),
			"ref": map[string]any{
				"tag": "tag2",
			},
		},
	}))
	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"git": map[string]any{
			"url": ip1.GitUrl(),
			"ref": "branch3",
		},
	}))
	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"git": map[string]any{
			"url": ip1.GitUrl(),
			"ref": "tag4",
		},
	}))
	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"git": map[string]any{
			"url": ip1.GitUrl(),
			"ref": map[string]any{
				"tag": "tag5",
			},
		},
	}))
	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"git": map[string]any{
			"url": ip1.GitUrl(),
			"ref": map[string]any{
				"commit": commit6,
			},
		},
	}))

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "parent")
	assertConfigMapExists(t, k, p.TestSlug(), "head")
	assertConfigMapExists(t, k, p.TestSlug(), "branch1")
	assertConfigMapExists(t, k, p.TestSlug(), "tag2")
	assertConfigMapExists(t, k, p.TestSlug(), "branch3")
	assertConfigMapExists(t, k, p.TestSlug(), "tag4")
	assertConfigMapExists(t, k, p.TestSlug(), "tag5")
	assertConfigMapExists(t, k, p.TestSlug(), "commit6")
}

func TestLocalGitOverride(t *testing.T) {
	t.Parallel()

	k := defaultCluster1
	p, ip1, ip2 := prepareGitIncludeTest(t, k, nil, nil, nil)

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
	k1 := u1.RepoKey().String()
	k2 := u2.RepoKey().String()

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
	gs := git2.NewTestGitServer(t)
	p, ip1, ip2 := prepareGitIncludeTest(t, k, gs, gs, gs)

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
	k1 := u1.RepoKey().String()

	p.KluctlMust("deploy", "--yes", "-t", "test",
		"--local-git-group-override", fmt.Sprintf("%s=%s", k1, overrideGroupDir),
	)
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "o1", "data", "a")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include2-cm")
	assertNestedFieldEquals(t, cm, "o2", "data", "a")
}
