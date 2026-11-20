package e2e

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Regression test for https://github.com/kluctl/kluctl/issues/1589:
// --abort-on-error must also stop applying the remaining hooks of the same
// batch once a hook failed (here: timed out while waiting for readiness).
func TestHooksAbortOnError(t *testing.T) {
	t.Parallel()

	s := prepareHookTestProjectBase(t)

	s.p.AddKustomizeDeployment("hook", nil, nil)

	s.addConfigMap("hook", resourceOpts{name: "cm1", namespace: s.p.TestSlug()})

	// hook1 never gets ready and runs into its hook timeout, which must abort the
	// deployment due to --abort-on-error
	s.addHookConfigMap("hook", resourceOpts{name: "hook1", namespace: s.p.TestSlug(), annotations: map[string]string{
		"kluctl.io/is-ready":     "false",
		"kluctl.io/hook-weight":  "1",
		"kluctl.io/hook-timeout": "3s",
	}}, false, "pre-deploy", "")

	// hook2 has a higher weight and thus would be applied after hook1. It must not
	// be applied anymore once hook1 aborted the deployment. cm1 proves that the
	// regular (non-hook) objects are skipped as well.
	s.addHookConfigMap("hook", resourceOpts{name: "hook2", namespace: s.p.TestSlug(), annotations: map[string]string{
		"kluctl.io/hook-weight": "2",
	}}, false, "pre-deploy", "")

	s.clearSeenConfigmaps()
	s.incRunCount()
	_, stderr, err := s.p.Kluctl(t, "deploy", "--yes", "-t", "test", "--abort-on-error")
	assert.Error(t, err)
	assert.Contains(t, stderr, "timed out while waiting for readiness of")

	// hook1 was applied (and then timed out), but hook2 and cm1 must not have been
	// applied anymore
	assert.Equal(t, []string{"hook1"}, s.seenConfigMaps)
}
