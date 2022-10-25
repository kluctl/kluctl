package e2e

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"testing"

	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
)

type HookTestSuite struct {
	suite.Suite

	k *test_utils.EnvTestCluster

	seenConfigMaps []string
}

func TestHooks(t *testing.T) {
	t.Parallel()
	suite.Run(t, &HookTestSuite{})
}

func (s *HookTestSuite) SetupSuite() {
	s.clearSeenConfigmaps()

	s.k = test_utils.CreateEnvTestCluster("cluster1")
	s.k.AddWebhookCallback(schema.GroupVersionResource{
		Version: "v1", Resource: "configmaps",
	}, true, s.handleConfigmap)
	err := s.k.Start()
	if err != nil {
		s.T().Fatal(err)
	}
}

func (s *HookTestSuite) TearDownSuite() {
	s.k.Stop()
}

func (s *HookTestSuite) SetupTest() {
	s.clearSeenConfigmaps()
}

func (s *HookTestSuite) handleConfigmap(request admission.Request) {
	x, err := uo.FromString(string(request.Object.Raw))
	if err != nil {
		s.T().Fatal(err)
	}
	generation, _, err := x.GetNestedInt("metadata", "generation")
	if err != nil {
		s.T().Fatal(err)
	}
	uid, _, err := x.GetNestedString("metadata", "uid")
	if err != nil {
		s.T().Fatal(err)
	}

	s.T().Logf("handleConfigmap: op=%s, name=%s/%s, generation=%d, uid=%s", request.Operation, request.Namespace, request.Name, generation, uid)

	s.seenConfigMaps = append(s.seenConfigMaps, request.Name)
}

func (s *HookTestSuite) clearSeenConfigmaps() {
	s.T().Logf("clearSeenConfigmaps: %v", s.seenConfigMaps)
	s.seenConfigMaps = nil
}

func (s *HookTestSuite) addHookConfigMap(p *testProject, dir string, opts resourceOpts, isHelm bool, hook string, hookDeletionPolicy string) {
	annotations := make(map[string]string)
	if isHelm {
		annotations["helm.sh/hook"] = hook
		if hookDeletionPolicy != "" {
			annotations["helm.sh/hook-deletion-policy"] = hookDeletionPolicy
		}
	} else {
		annotations["kluctl.io/hook"] = hook
	}
	if hookDeletionPolicy != "" {
		annotations["kluctl.io/hook-deletion-policy"] = hookDeletionPolicy
	}

	opts.annotations = uo.CopyMergeStrMap(opts.annotations, annotations)

	s.addConfigMap(p, dir, opts)
}

func (s *HookTestSuite) addConfigMap(p *testProject, dir string, opts resourceOpts) {
	o := uo.New()
	o.SetK8sGVKs("", "v1", "ConfigMap")
	mergeMetadata(o, opts)
	o.SetNestedField(map[string]interface{}{}, "data")
	p.addKustomizeResources(dir, []kustomizeResource{
		{fmt.Sprintf("%s.yml", opts.name), "", o},
	})
}

func (s *HookTestSuite) prepareHookTestProject(name string, hook string, hookDeletionPolicy string) *testProject {
	isDone := false
	namespace := fmt.Sprintf("hook-%s", name)
	p := &testProject{}
	p.init(s.T(), s.k, namespace)
	defer func() {
		if !isDone {
			p.cleanup()
		}
	}()

	createNamespace(s.T(), s.k, namespace)

	p.updateTarget("test", nil)

	p.addKustomizeDeployment("hook", nil, nil)

	s.addConfigMap(p, "hook", resourceOpts{name: "cm1", namespace: namespace})
	s.addHookConfigMap(p, "hook", resourceOpts{name: "hook1", namespace: namespace}, false, hook, hookDeletionPolicy)

	isDone = true
	return p
}

func (s *HookTestSuite) ensureHookExecuted(p *testProject, expectedCms ...string) {
	s.clearSeenConfigmaps()
	p.KluctlMust("deploy", "--yes", "-t", "test")
	assert.Equal(s.T(), expectedCms, s.seenConfigMaps)
}

func (s *HookTestSuite) ensureHookNotExecuted(p *testProject) {
	_, _, _ = s.k.Kubectl("delete", "-n", p.projectName, "configmaps", "cm1")
	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertResourceNotExists(s.T(), s.k, p.projectName, "ConfigMap/cm1")
}

func (s *HookTestSuite) TestHooksPreDeployInitial() {
	p := s.prepareHookTestProject("pre-deploy-initial", "pre-deploy-initial", "")
	defer p.cleanup()
	s.ensureHookExecuted(p, "hook1", "cm1")
	s.ensureHookExecuted(p, "cm1")
}

func (s *HookTestSuite) TestHooksPostDeployInitial() {
	p := s.prepareHookTestProject("post-deploy-initial", "post-deploy-initial", "")
	defer p.cleanup()
	s.ensureHookExecuted(p, "cm1", "hook1")
	s.ensureHookExecuted(p, "cm1")
}

func (s *HookTestSuite) TestHooksPreDeployUpgrade() {
	p := s.prepareHookTestProject("pre-deploy-upgrade", "pre-deploy-upgrade", "")
	defer p.cleanup()
	s.ensureHookExecuted(p, "cm1")
	s.ensureHookExecuted(p, "hook1", "cm1")
	s.ensureHookExecuted(p, "hook1", "cm1")
}

func (s *HookTestSuite) TestHooksPostDeployUpgrade() {
	p := s.prepareHookTestProject("post-deploy-upgrade", "post-deploy-upgrade", "")
	defer p.cleanup()
	s.ensureHookExecuted(p, "cm1")
	s.ensureHookExecuted(p, "cm1", "hook1")
	s.ensureHookExecuted(p, "cm1", "hook1")
}

func (s *HookTestSuite) doTestHooksPreDeploy(name string, hooks string) {
	p := s.prepareHookTestProject(name, hooks, "")
	defer p.cleanup()
	s.ensureHookExecuted(p, "hook1", "cm1")
	s.ensureHookExecuted(p, "hook1", "cm1")
}

func (s *HookTestSuite) doTestHooksPostDeploy(name string, hooks string) {
	p := s.prepareHookTestProject(name, hooks, "")
	defer p.cleanup()
	s.ensureHookExecuted(p, "cm1", "hook1")
	s.ensureHookExecuted(p, "cm1", "hook1")
}

func (s *HookTestSuite) doTestHooksPrePostDeploy(name string, hooks string) {
	p := s.prepareHookTestProject(name, hooks, "")
	defer p.cleanup()
	s.ensureHookExecuted(p, "hook1", "cm1", "hook1")
	s.ensureHookExecuted(p, "hook1", "cm1", "hook1")
}

func (s *HookTestSuite) TestHooksPreDeploy() {
	s.doTestHooksPreDeploy("pre-deploy", "pre-deploy")
}

func (s *HookTestSuite) TestHooksPreDeploy2() {
	// same as pre-deploy
	s.doTestHooksPreDeploy("pre-deploy2", "pre-deploy-initial,pre-deploy-upgrade")
}

func (s *HookTestSuite) TestHooksPostDeploy() {
	s.doTestHooksPostDeploy("post-deploy", "post-deploy")
}

func (s *HookTestSuite) TestHooksPostDeploy2() {
	// same as post-deploy
	s.doTestHooksPostDeploy("post-deploy2", "post-deploy-initial,post-deploy-upgrade")
}

func (s *HookTestSuite) TestHooksPrePostDeploy() {
	s.doTestHooksPrePostDeploy("pre-post-deploy", "pre-deploy,post-deploy")
}

func (s *HookTestSuite) TestHooksPrePostDeploy2() {
	s.doTestHooksPrePostDeploy("pre-post-deploy2", "pre-deploy-initial,pre-deploy-upgrade,post-deploy-initial,post-deploy-upgrade")
}
