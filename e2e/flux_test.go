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

	test_resources.ApplyYaml("flux.yaml", k)
	test_resources.ApplyYaml("kluctl.yaml", k)
	assertResourceExists(t, k, "flux-system", "kluctldeployment/microservices-demo-test")

	// p.KluctlMust("flux-suspend", "--namespace", "flux-system", "--kluctl-deployment", "microservices-demo-test")

	// p.KluctlMust("flux-resume", "--namespace", "flux-system", "--kluctl-deployment", "microservices-demo-test")
	p.KluctlMust("flux-reconcile", "--namespace", "flux-system", "--kluctl-deployment", "microservices-demo-test", "--with-source")

}
