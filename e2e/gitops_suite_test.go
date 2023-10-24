package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/huandu/xstrings"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/meta"
	"github.com/onsi/gomega"
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

	cancelController context.CancelFunc

	gitopsNamespace        string
	gitopsResultsNamespace string
	gitopsSecretIdx        int
	deletedDeployments     []client.ObjectKey
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
	g := NewWithT(suite.T())

	g.Eventually(func() bool {
		for _, key := range suite.deletedDeployments {
			var kd kluctlv1.KluctlDeployment
			err := suite.k.Client.Get(context.TODO(), key, &kd)
			if err == nil {
				return false
			}
		}
		return true
	}, timeout, time.Second).Should(BeTrue())

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
		"--metrics-bind-address=0",
		"--health-probe-bind-address=0",
	}
	done := make(chan struct{})
	go func() {
		_, _, err := test_project.KluctlExecute(suite.T(), ctx, args...)
		if err != nil {
			suite.T().Error(err)
		}
		close(done)
	}()

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
		a[kluctlv1.KluctlRequestReconcileAnnotation] = reconcileId
		kd.SetAnnotations(a)
	})
	return reconcileId
}

func (suite *GitopsTestSuite) waitForReconcile(key client.ObjectKey) {
	g := gomega.NewWithT(suite.T())

	reconcileId := suite.triggerReconcile(key)

	suite.T().Logf("%s: waiting for reconcile to finish", reconcileId)

	g.Eventually(func() bool {
		var kd kluctlv1.KluctlDeployment
		err := suite.k.Client.Get(context.TODO(), key, &kd)
		g.Expect(err).To(Succeed())
		if kd.Status.ReconcileRequestResult == nil || kd.Status.ReconcileRequestResult.RequestValue != reconcileId {
			suite.T().Logf("%s: request processing not started yet", reconcileId)
			return false
		}
		if kd.Status.ReconcileRequestResult.EndTime == nil {
			suite.T().Logf("%s: request processing not finished yet", reconcileId)
			return false
		}
		suite.T().Logf("%s: finished waiting", reconcileId)
		return true
	}, timeout, time.Second).Should(BeTrue())
}

func (suite *GitopsTestSuite) waitForCommit(key client.ObjectKey, commit string) {
	g := gomega.NewWithT(suite.T())

	reconcileId := suite.triggerReconcile(key)

	suite.T().Logf("%s: waiting for commit %s", reconcileId, commit)

	g.Eventually(func() bool {
		var kd kluctlv1.KluctlDeployment
		err := suite.k.Client.Get(context.Background(), key, &kd)
		g.Expect(err).To(Succeed())
		if kd.Status.ReconcileRequestResult == nil || kd.Status.ReconcileRequestResult.RequestValue != reconcileId {
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
		suite.T().Logf("%s: finished waiting", reconcileId)
		return true
	}, timeout, time.Second).Should(BeTrue())
}

func (suite *GitopsTestSuite) createKluctlDeployment(p *test_project.TestProject, target string, args map[string]any) client.ObjectKey {
	return suite.createKluctlDeployment2(p, target, args, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.Source.Git = &kluctlv1.ProjectSourceGit{
			URL: *types2.ParseGitUrlMust(p.GitUrl()),
		}
	})
}

func (suite *GitopsTestSuite) createKluctlDeployment2(p *test_project.TestProject, target string, args map[string]any, modify ...func(kd *kluctlv1.KluctlDeployment)) client.ObjectKey {
	createNamespace(suite.T(), suite.k, suite.gitopsNamespace)

	jargs, err := json.Marshal(args)
	if err != nil {
		suite.T().Fatal(err)
	}

	kluctlDeployment := &kluctlv1.KluctlDeployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      p.TestSlug(),
			Namespace: suite.gitopsNamespace,
		},
		Spec: kluctlv1.KluctlDeploymentSpec{
			Interval: metav1.Duration{Duration: interval},
			Timeout:  &metav1.Duration{Duration: timeout},
			Target:   &target,
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

func (suite *GitopsTestSuite) updateKluctlDeployment(key client.ObjectKey, update func(kd *kluctlv1.KluctlDeployment)) *kluctlv1.KluctlDeployment {
	g := NewWithT(suite.T())

	var kd kluctlv1.KluctlDeployment
	err := suite.k.Client.Get(context.TODO(), key, &kd)
	g.Expect(err).To(Succeed())

	patch := client.MergeFrom(kd.DeepCopy())

	update(&kd)

	err = suite.k.Client.Patch(context.TODO(), &kd, patch, client.FieldOwner("kubectl"))
	g.Expect(err).To(Succeed())

	return &kd
}

func (suite *GitopsTestSuite) deleteKluctlDeployment(key client.ObjectKey) {
	g := NewWithT(suite.T())

	var kd kluctlv1.KluctlDeployment
	kd.Name = key.Name
	kd.Namespace = key.Namespace
	err := suite.k.Client.Delete(context.Background(), &kd)
	if err != nil && !errors.IsNotFound(err) {
		g.Expect(err).To(Succeed())
	}

	suite.deletedDeployments = append(suite.deletedDeployments, key)
}

func (suite *GitopsTestSuite) createGitopsSecret(m map[string]string) string {
	g := NewWithT(suite.T())

	name := fmt.Sprintf("gitops-seceet-%d", suite.gitopsSecretIdx)
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
