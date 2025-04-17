package e2e

import (
	"fmt"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/e2e/test_resources"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func createExampleCR(name string, namespace string) string {
	return fmt.Sprintf(`
apiVersion: "stable.example.com/v1"
kind: CronTab
metadata:
  name: %s
  namespace: %s
spec:
  cronSpec: "* * * * */5"
  image: my-awesome-cron-image

`, name, namespace)
}

func prepareCRDsTest(t *testing.T, k *test_utils.EnvTestCluster, crds bool, barrier bool) *test_project.TestProject {
	p := test_project.NewTestProject(t)
	p.AddExtraArgs("--kubeconfig", getKubeconfigTmpFile(t, k.Kubeconfig))

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test1", func(target *uo.UnstructuredObject) {
	})

	if crds {
		p.AddKustomizeDeployment("crds", []test_project.KustomizeResource{
			{Name: "crds.yaml", Content: test_resources.GetYamlDocs(t, "example-crds.yaml")},
		}, nil)
	}
	if barrier {
		p.AddDeploymentItem(".", uo.FromMap(map[string]interface{}{
			"barrier": true,
		}))
	}
	p.AddKustomizeDeployment("crs", []test_project.KustomizeResource{
		{Name: "crs.yaml", Content: createExampleCR("test", p.TestSlug())},
	}, nil)

	return p
}

func TestDeployCRDUnordered(t *testing.T) {
	t.Parallel()

	for i := 0; i < 100; i++ {
		success := func() bool {
			k := createTestCluster(t, "cluster1")
			defer k.Stop()

			p := prepareCRDsTest(t, k, true, false)

			stdout, _, err := p.Kluctl(t, "deploy", "--yes", "-t", "test1")
			if err != nil && strings.Contains(err.Error(), "command failed") && strings.Contains(stdout, `no matches for kind "CronTab" in version`) {
				// success
				return true
			}
			return false
		}()
		if success {
			return
		}
	}

	t.Errorf("could not cause missing CRD error")
}

func TestDiffCRDSimulated(t *testing.T) {
	t.Parallel()

	k := createTestCluster(t, "cluster1")

	p := prepareCRDsTest(t, k, true, true)

	stdout, _ := p.KluctlMust(t, "diff", "-t", "test1")
	assert.Contains(t, stdout, fmt.Sprintf("the underyling custom resource definition for %s/CronTab/test has not been applied yet", p.TestSlug()))
}

func TestDeployCRDBarrier(t *testing.T) {
	t.Parallel()

	k := createTestCluster(t, "cluster1")

	p := prepareCRDsTest(t, k, true, true)

	p.KluctlMust(t, "deploy", "--yes", "-t", "test1")
}

func TestDeployCRDByController(t *testing.T) {
	t.Parallel()

	k := createTestCluster(t, "cluster1")

	p := prepareCRDsTest(t, k, false, true)
	p.UpdateDeploymentItems(".", func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
		barrierItem := items[0]
		barrierItem.SetNestedField([]map[string]any{
			{
				"kind": "CustomResourceDefinition",
				"name": "crontabs.stable.example.com",
			},
		}, "waitReadinessObjects")
		return items
	})

	// we simulate a controller being spun up that needs a few seconds to get ready and then apply the CRDs
	go func() {
		time.Sleep(5 * time.Second)
		test_resources.ApplyYaml(t, "example-crds.yaml", k)
	}()

	p.KluctlMust(t, "deploy", "--yes", "-t", "test1")
}
