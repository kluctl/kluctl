package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/huandu/xstrings"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	port_tool "github.com/kluctl/kluctl/v2/e2e/test-utils/port-tool"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/sourceoverride"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/meta"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"math/rand"
	"os"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"time"

	. "github.com/onsi/gomega"
)

const (
	timeout  = time.Second * 300
	interval = time.Second * 5
)

func init() {
	// this must be called in the first 30 seconds of startup, so we have to do it here at init() time
	ctrl.SetLogger(klog.NewKlogr())
}

type GitopsTestSuite struct {
	suite.Suite

	k *test_utils.EnvTestCluster

	sourceOverridePort int
	cancelController   context.CancelFunc

	gitopsNamespace        string
	gitopsResultsNamespace string
	gitopsSecretIdx        int
}

func (suite *GitopsTestSuite) SetupSuite() {
	suite.k = gitopsCluster

	n := suite.T().Name()
	n = xstrings.ToKebabCase(n)
	n = strings.ReplaceAll(n, "/", "-")
	n += "-gitops"
	suite.gitopsNamespace = n
	suite.gitopsResultsNamespace = n + "-r"

	createNamespace(suite.T(), suite.k, suite.gitopsNamespace)

	suite.startController()
}

func (suite *GitopsTestSuite) TearDownSuite() {
	if suite.cancelController != nil {
		suite.cancelController()
	}
}

func (suite *GitopsTestSuite) TearDownTest() {
}

func (suite *GitopsTestSuite) startController() {
	tmpKubeconfig := filepath.Join(suite.T().TempDir(), "kubeconfig")
	err := os.WriteFile(tmpKubeconfig, suite.k.Kubeconfig, 0o600)
	if err != nil {
		suite.T().Fatal(err)
	}

	controllerNamespace := suite.gitopsNamespace + "-system"
	createNamespace(suite.T(), suite.k, controllerNamespace)

	suite.sourceOverridePort = port_tool.NextFreePort("127.0.0.1")

	ctx, ctxCancel := context.WithCancel(context.Background())
	args := []string{
		"controller",
		"run",
		"--kubeconfig",
		tmpKubeconfig,
		"--context",
		"gitops",
		"--namespace",
		suite.gitopsNamespace,
		"--command-result-namespace",
		suite.gitopsNamespace + "-results",
		"--controller-namespace",
		controllerNamespace,
		"--metrics-bind-address=0",
		"--health-probe-bind-address=0",
		fmt.Sprintf("--source-override-bind-address=localhost:%d", suite.sourceOverridePort),
	}
	done := make(chan struct{})
	go func() {
		logFn := func(args ...any) {
			suite.T().Log(args...)
		}
		_, _, err := test_project.KluctlExecute(suite.T(), ctx, logFn, args...)
		if err != nil {
			suite.T().Error(err)
		}
		close(done)
	}()

	_, _, _, err = sourceoverride.WaitAndLoadSecret(ctx, suite.k.Client, controllerNamespace)
	if err != nil {
		suite.T().Fatal(err)
	}

	cancel := func() {
		ctxCancel()
		<-done
	}
	suite.cancelController = cancel
}

func (suite *GitopsTestSuite) triggerReconcile(key client.ObjectKey) string {
	reconcileId := fmt.Sprintf("%d", rand.Int63())

	suite.T().Logf("%s: triggerReconcile", reconcileId)

	suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
		a := kd.GetAnnotations()
		if a == nil {
			a = map[string]string{}
		}
		mr := kluctlv1.ManualRequest{
			RequestValue: reconcileId,
		}
		a[kluctlv1.KluctlRequestReconcileAnnotation] = yaml.WriteJsonStringMust(&mr)
		kd.SetAnnotations(a)
	})
	return reconcileId
}

func (suite *GitopsTestSuite) waitForReconcile(key client.ObjectKey) *kluctlv1.KluctlDeployment {
	g := gomega.NewWithT(suite.T())

	reconcileId := suite.triggerReconcile(key)

	suite.T().Logf("%s: waiting for reconcile to finish", reconcileId)

	var kd *kluctlv1.KluctlDeployment
	g.Eventually(func() bool {
		kd = suite.getKluctlDeployment(key)
		if kd.Status.ReconcileRequestResult == nil || kd.Status.ReconcileRequestResult.Request.RequestValue != reconcileId {
			suite.T().Logf("%s: request processing not started yet", reconcileId)
			return false
		}
		if kd.Status.ReconcileRequestResult.EndTime == nil {
			suite.T().Logf("%s: request processing not finished yet", reconcileId)
			return false
		}
		readinessCondition := suite.getReadiness(kd)
		if readinessCondition == nil || readinessCondition.Status == metav1.ConditionUnknown {
			suite.T().Logf("%s: readiness status == unknown", reconcileId)
			return false
		}
		suite.T().Logf("%s: finished waiting", reconcileId)
		return true
	}, timeout, time.Second).Should(BeTrue())
	return kd
}

func (suite *GitopsTestSuite) waitForCommit(key client.ObjectKey, commit string) *kluctlv1.KluctlDeployment {
	g := gomega.NewWithT(suite.T())

	reconcileId := suite.triggerReconcile(key)

	suite.T().Logf("%s: waiting for commit %s", reconcileId, commit)

	var kd *kluctlv1.KluctlDeployment
	g.Eventually(func() bool {
		kd = suite.getKluctlDeployment(key)
		if kd.Status.ReconcileRequestResult == nil || kd.Status.ReconcileRequestResult.Request.RequestValue != reconcileId {
			suite.T().Logf("%s: request processing not started yet", reconcileId)
			return false
		}
		if kd.Status.ReconcileRequestResult.EndTime == nil {
			suite.T().Logf("%s: request processing not finished yet", reconcileId)
			return false
		}
		if kd.Status.ObservedCommit != commit {
			suite.T().Logf("%s: commit %s does mot match %s", reconcileId, kd.Status.ObservedCommit, commit)
			return false
		}
		readinessCondition := suite.getReadiness(kd)
		if readinessCondition == nil || readinessCondition.Status == metav1.ConditionUnknown {
			suite.T().Logf("%s: readiness status == unknown", reconcileId)
			return false
		}
		suite.T().Logf("%s: finished waiting", reconcileId)
		return true
	}, timeout, time.Second).Should(BeTrue())
	return kd
}

func (suite *GitopsTestSuite) createKluctlDeployment(p *test_project.TestProject, target string, args map[string]any) client.ObjectKey {
	return suite.createKluctlDeployment2(p, target, args, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.Source.Git = &kluctlv1.ProjectSourceGit{
			URL: p.GitUrl(),
		}
	})
}

func (suite *GitopsTestSuite) createKluctlDeployment2(p *test_project.TestProject, target string, args map[string]any, modify ...func(kd *kluctlv1.KluctlDeployment)) client.ObjectKey {
	createNamespace(suite.T(), suite.k, suite.gitopsNamespace)

	jargs, err := json.Marshal(args)
	if err != nil {
		suite.T().Fatal(err)
	}

	var targetPtr *string
	if target != "" {
		targetPtr = &target
	}

	kluctlDeployment := &kluctlv1.KluctlDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.TestSlug(),
			Namespace: suite.gitopsNamespace,
		},
		Spec: kluctlv1.KluctlDeploymentSpec{
			Interval: metav1.Duration{Duration: interval},
			Timeout:  &metav1.Duration{Duration: timeout},
			Target:   targetPtr,
			Args: &runtime.RawExtension{
				Raw: jargs,
			},
		},
	}

	for _, m := range modify {
		if m != nil {
			m(kluctlDeployment)
		}
	}

	err = suite.k.Client.Create(context.Background(), kluctlDeployment)
	if err != nil {
		suite.T().Fatal(err)
	}

	key := client.ObjectKeyFromObject(kluctlDeployment)
	suite.T().Cleanup(func() {
		suite.deleteKluctlDeployment(key)
	})
	return key
}

func (suite *GitopsTestSuite) getKluctlDeploymentAllowNil(key client.ObjectKey) *kluctlv1.KluctlDeployment {
	var kd kluctlv1.KluctlDeployment
	err := suite.k.Client.Get(context.TODO(), key, &kd)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		suite.T().Fatal(err)
	}
	return &kd
}

func (suite *GitopsTestSuite) getKluctlDeployment(key client.ObjectKey) *kluctlv1.KluctlDeployment {
	kd := suite.getKluctlDeploymentAllowNil(key)
	if kd == nil {
		suite.T().Fatal(fmt.Sprintf("KluctlDeployment %s not found", key.String()))
	}
	return kd
}

func (suite *GitopsTestSuite) updateKluctlDeployment(key client.ObjectKey, update func(kd *kluctlv1.KluctlDeployment)) *kluctlv1.KluctlDeployment {
	g := NewWithT(suite.T())

	kd := suite.getKluctlDeployment(key)

	patch := client.MergeFrom(kd.DeepCopy())

	update(kd)

	err := suite.k.Client.Patch(context.TODO(), kd, patch, client.FieldOwner("kubectl"))
	g.Expect(err).To(Succeed())

	return kd
}

func (suite *GitopsTestSuite) deleteKluctlDeployment(key client.ObjectKey) {
	g := NewWithT(suite.T())

	suite.T().Logf("Deleting KluctlDeployment %s", key.String())

	var kd kluctlv1.KluctlDeployment
	kd.Name = key.Name
	kd.Namespace = key.Namespace
	err := suite.k.Client.Delete(context.Background(), &kd)
	if err != nil && !errors.IsNotFound(err) {
		g.Expect(err).To(Succeed())
	}

	g.Eventually(func() bool {
		if suite.getKluctlDeploymentAllowNil(key) != nil {
			return false
		}
		return true
	}, timeout, time.Second).Should(BeTrue())

	suite.T().Logf("KluctlDeployment %s has vanished", key.String())
}

func (suite *GitopsTestSuite) createGitopsSecret(m map[string]string) string {
	g := NewWithT(suite.T())

	name := fmt.Sprintf("gitops-secret-%d", suite.gitopsSecretIdx)
	suite.gitopsSecretIdx++

	mb := map[string][]byte{}
	for k, v := range m {
		mb[k] = []byte(v)
	}
	credsSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: suite.gitopsNamespace,
			Name:      name,
		},
		Data: mb,
	}
	err := suite.k.Client.Create(context.TODO(), credsSecret)
	g.Expect(err).To(Succeed())

	return name
}

func (suite *GitopsTestSuite) getReadiness(obj *kluctlv1.KluctlDeployment) *metav1.Condition {
	for _, c := range obj.Status.Conditions {
		if c.Type == meta.ReadyCondition {
			return &c
		}
	}
	return nil
}

func (suite *GitopsTestSuite) getCommandResult(id string) *result.CommandResult {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rs, err := results.NewResultStoreSecrets(ctx, suite.k.RESTConfig(), suite.k.Client, false, "", 0, 0)
	assert.NoError(suite.T(), err)

	cr, err := rs.GetCommandResult(results.GetCommandResultOptions{Id: id})
	if err != nil {
		return nil
	}
	return cr
}

func (suite *GitopsTestSuite) getValidateResult(id string) *result.ValidateResult {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rs, err := results.NewResultStoreSecrets(ctx, suite.k.RESTConfig(), suite.k.Client, false, "", 0, 0)
	assert.NoError(suite.T(), err)

	vr, err := rs.GetValidateResult(results.GetValidateResultOptions{Id: id})
	if err != nil {
		return nil
	}
	return vr
}

func (suite *GitopsTestSuite) assertNoChanges(resultId string) {
	cr := suite.getCommandResult(resultId)
	assert.NotNil(suite.T(), cr)

	sum := cr.BuildSummary()
	assert.Zero(suite.T(), sum.NewObjects)
	assert.Zero(suite.T(), sum.ChangedObjects)
	assert.Zero(suite.T(), sum.OrphanObjects)
	assert.Zero(suite.T(), sum.DeletedObjects)
}

func (suite *GitopsTestSuite) assertChanges(resultId string, new int, changed int, orphan int, deleted int) {
	cr := suite.getCommandResult(resultId)
	assert.NotNil(suite.T(), cr)

	sum := cr.BuildSummary()
	assert.Equal(suite.T(), new, sum.NewObjects)
	assert.Equal(suite.T(), changed, sum.ChangedObjects)
	assert.Equal(suite.T(), orphan, sum.OrphanObjects)
	assert.Equal(suite.T(), deleted, sum.DeletedObjects)
}
