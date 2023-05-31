package e2e

import (
	"context"
	"fmt"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_resources"
	meta2 "github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/meta"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)
import . "github.com/onsi/gomega"

var legacyGvk = schema.GroupVersionKind{
	Group:   "flux.kluctl.io",
	Version: "v1alpha1",
	Kind:    "KluctlDeployment",
}

func (suite *GitopsTestSuite) createLegacyKluctlDeployment(p *test_utils.TestProject, target string) client.ObjectKey {
	gitopsNs := p.TestSlug() + "-gitops"
	createNamespace(suite.T(), suite.k, gitopsNs)

	kluctlDeployment := uo.New()
	kluctlDeployment.SetK8sGVK(legacyGvk)

	kluctlDeployment.SetK8sName(p.TestSlug())
	kluctlDeployment.SetK8sNamespace(gitopsNs)
	_ = kluctlDeployment.SetNestedField(map[string]any{
		"interval":       interval.String(),
		"deployInterval": "never",
		"timeout":        timeout.String(),
		"target":         target,
		"source": map[string]any{
			"url": p.GitUrl(),
		},
	}, "spec")

	err := suite.k.Client.Create(context.Background(), kluctlDeployment.ToUnstructured())
	if err != nil {
		suite.T().Fatal(err)
	}

	key := client.ObjectKeyFromObject(kluctlDeployment.ToUnstructured())
	return key
}

func (suite *GitopsTestSuite) TestGitOpsLegacyMigration() {
	g := NewWithT(suite.T())

	test_resources.ApplyYaml(suite.T(), "flux.kluctl.io_kluctldeployments.yaml", suite.k)
	suite.T().Cleanup(func() {
		obj := uo.New()
		obj.SetK8sGVK(apiextensionsv1.SchemeGroupVersion.WithKind("CustomResourceDefinition"))
		obj.SetK8sName("kluctldeployments.flux.kluctl.io")
		_ = suite.k.Client.Delete(context.TODO(), obj.ToUnstructured())
	})

	p := test_utils.NewTestProject(suite.T())
	createNamespace(suite.T(), suite.k, p.TestSlug())

	p.UpdateTarget("target1", nil)
	p.AddKustomizeDeployment("d1", []test_utils.KustomizeResource{
		{Name: "cm1.yaml", Content: uo.FromStringMust(fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: "%s"
data:
  k1: v1
`, p.TestSlug()))},
	}, nil)

	key := suite.createLegacyKluctlDeployment(p, "target1")
	key2 := suite.createKluctlDeployment(p, "target1", nil)
	assert.Equal(suite.T(), key, key2)

	checkMigrationStatus := func() bool {
		var kd kluctlv1.KluctlDeployment
		_ = suite.k.Client.Get(context.Background(), key, &kd)
		c := meta.FindStatusCondition(kd.Status.Conditions, meta2.ReadyCondition)
		return c != nil && c.Reason == kluctlv1.WaitingForLegacyMigrationReason
	}

	suite.Run("wait for migration to start", func() {
		g.Eventually(checkMigrationStatus, timeout, time.Second).Should(BeTrue())
	})

	suite.Run("stays in migration state", func() {
		suite.triggerReconcile(key)
		g.Consistently(checkMigrationStatus, 3*time.Second, time.Second).Should(BeTrue())
	})

	suite.Run("legacy controller marks the deployment with readyForMigration", func() {
		var obj unstructured.Unstructured
		obj.SetGroupVersionKind(legacyGvk)
		err := suite.k.Client.Get(context.TODO(), key, &obj)
		g.Expect(err).To(Succeed())

		_ = unstructured.SetNestedField(obj.Object, true, "status", "readyForMigration")
		err = suite.k.Client.Status().Update(context.TODO(), &obj)
		g.Expect(err).To(Succeed())
	})

	suite.Run("wait for migration to end", func() {
		g.Eventually(checkMigrationStatus, timeout, time.Second).Should(BeFalse())
	})

	suite.Run("initial deployment", func() {
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})
}
