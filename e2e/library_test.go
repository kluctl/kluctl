package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"testing"
)

func prepareLibraryProject(t *testing.T, prefix string, subDir string, cmData map[string]string, args []types.DeploymentArg) *test_project.TestProject {
	p := test_project.NewTestProject(t,
		test_project.WithGitSubDir(subDir),
		test_project.WithRepoName(fmt.Sprintf("repos/%s", prefix)),
	)
	addConfigMapDeployment(p, "cm", cmData, resourceOpts{
		name:      fmt.Sprintf("%s-cm", prefix),
		namespace: p.TestSlug(),
	})
	p.UpdateYaml(".kluctl-library.yaml", func(o *uo.UnstructuredObject) error {
		*o = *uo.FromMap(map[string]interface{}{
			"args": args,
		})
		return nil
	}, "")
	return p
}

func TestLibraryIncludePassVars(t *testing.T) {
	k := defaultCluster1

	p := test_project.NewTestProject(t)
	ip1 := prepareLibraryProject(t, "lib1", "", map[string]string{
		"a": `{{ get_var("k1", "na") }}`,
	}, []types.DeploymentArg{})
	ip2 := prepareLibraryProject(t, "lib2", "subDir", map[string]string{
		"a": `{{ get_var("k2", "na") }}`,
	}, []types.DeploymentArg{})

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {})

	// add vars to deployment.yaml
	p.UpdateDeploymentYaml("", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField([]types.VarsSource{
			{
				Values: uo.FromMap(map[string]interface{}{
					"k1": "v1",
					"k2": "v2",
				}),
			},
		}, "vars")
		return nil
	})

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

	// ensure vars were not passed
	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm1 := assertConfigMapExists(t, k, p.TestSlug(), "lib1-cm")
	cm2 := assertConfigMapExists(t, k, p.TestSlug(), "lib2-cm")
	assertNestedFieldEquals(t, cm1, "na", "data", "a")
	assertNestedFieldEquals(t, cm2, "na", "data", "a")

	// now let it pass vars to lib1
	p.UpdateDeploymentItems("", func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
		_ = items[0].SetNestedField(true, "passVars")
		return items
	})
	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm1 = assertConfigMapExists(t, k, p.TestSlug(), "lib1-cm")
	cm2 = assertConfigMapExists(t, k, p.TestSlug(), "lib2-cm")
	assertNestedFieldEquals(t, cm1, "v1", "data", "a")
	assertNestedFieldEquals(t, cm2, "na", "data", "a")

	// now remove .kluctl-library.yaml and let it auto-pass variables
	ip2.DeleteFile(".kluctl-library.yaml", "")
	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm1 = assertConfigMapExists(t, k, p.TestSlug(), "lib1-cm")
	cm2 = assertConfigMapExists(t, k, p.TestSlug(), "lib2-cm")
	assertNestedFieldEquals(t, cm1, "v1", "data", "a")
	assertNestedFieldEquals(t, cm2, "v2", "data", "a")
}

func TestLibraryIncludeArgs(t *testing.T) {
	k := defaultCluster1

	p := test_project.NewTestProject(t)
	ip1 := prepareLibraryProject(t, "lib1", "", map[string]string{
		"a": `{{ get_var("args.a", "na") }}`,
		"b": `{{ get_var("args.b", "na") }}`,
	}, []types.DeploymentArg{
		{
			Name:    "a",
			Default: &apiextensionsv1.JSON{Raw: []byte(`"def"`)},
		},
		{
			Name: "b",
		},
	})

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {})

	// no args passed
	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"git": map[string]any{
			"url": ip1.GitUrl(),
		},
	}))

	// ensure vars were not passed
	_, _, err := p.Kluctl(t, "deploy", "--yes", "-t", "test")
	assert.ErrorContains(t, err, "required argument b not set")

	// pass args
	p.UpdateDeploymentItems("", func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
		_ = items[0].SetNestedField(uo.FromMap(map[string]interface{}{
			"b": "b",
		}), "args")
		return items
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm1 := assertConfigMapExists(t, k, p.TestSlug(), "lib1-cm")
	assertNestedFieldEquals(t, cm1, "def", "data", "a")
	assertNestedFieldEquals(t, cm1, "b", "data", "b")
}
