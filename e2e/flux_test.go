package e2e

import (
	"testing"

	"github.com/kluctl/kluctl/v2/e2e/test_resources"
)

func TestFluxCommands(t *testing.T) {
	t.Parallel()

	k := defaultKindCluster1

	p := &testProject{}
	p.init(t, k, "simple")

	defer p.cleanup()

	test_resources.ApplyYaml("flux-source-crd.yaml", k)
	test_resources.ApplyYaml("kluctl-crds.yaml", k)
	test_resources.ApplyYaml("kluctl-deployment.yaml", k)

	assertResourceExists(t, k, "default", "kluctldeployment/microservices-demo-test")

	p.KluctlMust("flux-suspend", "--namespace", "default", "--kluctl-deployment", "microservices-demo-test")

	p.KluctlMust("flux-resume", "--namespace", "default", "--kluctl-deployment", "microservices-demo-test", "--no-wait")
	p.KluctlMust("flux-reconcile", "--namespace", "default", "--kluctl-deployment", "microservices-demo-test", "--with-source", "--no-wait")

}
