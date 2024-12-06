package e2e

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"testing"
)

func prepareIncludeProject(t *testing.T, prefix string, subDir string, gitServer *test_utils.TestGitServer) *test_project.TestProject {
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

func prepareGitIncludeTest(t *testing.T, k *test_utils.EnvTestCluster, mainGs *test_utils.TestGitServer, gs1 *test_utils.TestGitServer, gs2 *test_utils.TestGitServer) (*test_project.TestProject, *test_project.TestProject, *test_project.TestProject) {
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

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
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

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "parent")
	assertConfigMapExists(t, k, p.TestSlug(), "head")
	assertConfigMapExists(t, k, p.TestSlug(), "branch1")
	assertConfigMapExists(t, k, p.TestSlug(), "tag2")
	assertConfigMapExists(t, k, p.TestSlug(), "branch3")
	assertConfigMapExists(t, k, p.TestSlug(), "tag4")
	assertConfigMapExists(t, k, p.TestSlug(), "tag5")
	assertConfigMapExists(t, k, p.TestSlug(), "commit6")
}
