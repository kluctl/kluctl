package e2e

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

type GitOpsFieldManagerTestSuite struct {
	GitopsTestSuite
}

func TestGitOpsFieldManager(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GitOpsFieldManagerTestSuite))
}

func (suite *GitopsTestSuite) TestFieldManager() {
	g := NewWithT(suite.T())

	p := test_project.NewTestProject(suite.T())
	createNamespace(suite.T(), suite.k, p.TestSlug())

	p.UpdateTarget("target1", nil)
	p.AddKustomizeDeployment("d1", []test_project.KustomizeResource{
		{Name: "cm1.yaml", Content: uo.FromStringMust(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: "{{ args.namespace }}"
data:
  k1: v1
  k2: "{{ args.k2 + 1 }}"
`)},
	}, nil)

	key := suite.createKluctlDeployment(p, "target1", map[string]any{
		"namespace": p.TestSlug(),
		"k2":        42,
	})

	suite.Run("initial deployment", func() {
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})

	suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.DeployInterval = &kluctlv1.SafeDuration{Duration: metav1.Duration{Duration: interval}}
	})

	cm := &corev1.ConfigMap{}

	suite.Run("cm1 is deployed", func() {
		err := suite.k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
		g.Expect(cm.Data).To(HaveKeyWithValue("k1", "v1"))
		g.Expect(cm.Data).To(HaveKeyWithValue("k2", "43"))
	})

	suite.Run("cm1 is modified and restored", func() {
		cm.Data["k1"] = "v2"
		err := suite.k.Client.Update(context.TODO(), cm, client.FieldOwner("kubectl"))
		g.Expect(err).To(Succeed())

		g.Eventually(func() bool {
			err := suite.k.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "cm1",
				Namespace: p.TestSlug(),
			}, cm)
			g.Expect(err).To(Succeed())
			return cm.Data["k1"] == "v1"
		}, timeout, time.Second).Should(BeTrue())
	})

	suite.Run("cm1 gets a key added which is not modified by the controller", func() {
		cm.Data["k1"] = "v2"
		cm.Data["k3"] = "v3"
		err := suite.k.Client.Update(context.TODO(), cm, client.FieldOwner("kubectl"))
		g.Expect(err).To(Succeed())

		g.Eventually(func() bool {
			err := suite.k.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "cm1",
				Namespace: p.TestSlug(),
			}, cm)
			g.Expect(err).To(Succeed())
			return cm.Data["k1"] == "v1"
		}, timeout, time.Second).Should(BeTrue())

		g.Expect(cm.Data).To(HaveKeyWithValue("k3", "v3"))
	})

	suite.Run("cm1 gets modified with another field manager", func() {
		patch := client.MergeFrom(cm.DeepCopy())
		cm.Data["k1"] = "v2"

		err := suite.k.Client.Patch(context.TODO(), cm, patch, client.FieldOwner("test-field-manager"))
		g.Expect(err).To(Succeed())

		for i := 0; i < 2; i++ {
			suite.waitForReconcile(key)
		}

		err = suite.k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
		g.Expect(cm.Data).To(HaveKeyWithValue("k1", "v2"))
	})

	suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.ForceApply = true
	})

	suite.Run("forceApply is true and cm1 gets restored even with another field manager", func() {
		patch := client.MergeFrom(cm.DeepCopy())
		cm.Data["k1"] = "v2"

		err := suite.k.Client.Patch(context.TODO(), cm, patch, client.FieldOwner("test-field-manager"))
		g.Expect(err).To(Succeed())

		g.Eventually(func() bool {
			err := suite.k.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "cm1",
				Namespace: p.TestSlug(),
			}, cm)
			g.Expect(err).To(Succeed())
			return cm.Data["k1"] == "v1"
		}, timeout, time.Second).Should(BeTrue())
	})
}
