package e2e

import (
	"fmt"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"testing"
)

func prepareIncludeProject(t *testing.T, prefix string, subDir string) *test_utils.TestProject {
	p := test_utils.NewTestProject(t, test_utils.WithGitSubDir(subDir))
	addConfigMapDeployment(p, "cm", map[string]string{"a": "v"}, resourceOpts{
		name:      fmt.Sprintf("%s-cm", prefix),
		namespace: p.TestSlug(),
	})
	return p
}

func prepareGitIncludeTest(t *testing.T) (*test_utils.TestProject, *test_utils.TestProject, *test_utils.TestProject) {
	k := defaultCluster1

	p := test_utils.NewTestProject(t)
	ip1 := prepareIncludeProject(t, "include1", "")
	ip2 := prepareIncludeProject(t, "include2", "subDir")

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

	p, _, _ := prepareGitIncludeTest(t)

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertConfigMapExists(t, k, p.TestSlug(), "include2-cm")
}
