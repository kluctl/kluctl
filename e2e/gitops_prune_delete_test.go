package e2e

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

type GitOpsPruneDeleteSuite struct {
	GitopsTestSuite
}

func TestGitOpsPruneDelete(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GitOpsPruneDeleteSuite))
}

func (suite *GitOpsPruneDeleteSuite) TestKluctlDeploymentReconciler_Prune() {
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
`)},
	}, nil)
	p.AddKustomizeDeployment("d2", []test_project.KustomizeResource{
		{Name: "cm2.yaml", Content: uo.FromStringMust(`apiVersion: v1
kind: ConfigMap
metadata:
  name: cm2
  namespace: "{{ args.namespace }}"
data:
  k1: v1
`)},
	}, nil)

	key := suite.createKluctlDeployment(p, "target1", map[string]any{
		"namespace": p.TestSlug(),
	})

	suite.waitForCommit(key, getHeadRevision(suite.T(), p))

	cm := &corev1.ConfigMap{}

	suite.Run("cm1 and cm2 got deployed", func() {
		err := suite.k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
		err = suite.k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm2",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
	})

	p.UpdateDeploymentYaml("", func(o *uo.UnstructuredObject) error {
		_ = o.RemoveNestedField("deployments", 1)
		return nil
	})

	g.Eventually(func() bool {
		var obj kluctlv1.KluctlDeployment
		_ = suite.k.Client.Get(context.Background(), key, &obj)
		if obj.Status.LastDeployResult == nil {
			return false
		}
		ldr, err := obj.Status.GetLastDeployResult()
		g.Expect(err).To(Succeed())
		return ldr.GitInfo.Commit == getHeadRevision(suite.T(), p)
	}, timeout, time.Second).Should(BeTrue())

	suite.Run("cm1 and cm2 were not deleted", func() {
		err := suite.k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
		err = suite.k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm2",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
	})

	suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.Prune = true
	})

	suite.waitForReconcile(key)

	suite.Run("cm1 did not get deleted and cm2 got deleted", func() {
		err := suite.k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
		err = suite.k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm2",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(MatchError("configmaps \"cm2\" not found"))
	})
}

func (suite *GitOpsPruneDeleteSuite) doTestDelete(delete bool) {
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
`)},
	}, nil)

	key := suite.createKluctlDeployment(p, "target1", map[string]any{
		"namespace": p.TestSlug(),
	})

	suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.Delete = delete
	})

	suite.waitForCommit(key, getHeadRevision(suite.T(), p))

	cm := &corev1.ConfigMap{}

	suite.Run("cm1 got deployed", func() {
		err := suite.k.Client.Get(context.TODO(), client.ObjectKey{
			Name:      "cm1",
			Namespace: p.TestSlug(),
		}, cm)
		g.Expect(err).To(Succeed())
	})

	suite.deleteKluctlDeployment(key)

	g.Eventually(func() bool {
		var obj kluctlv1.KluctlDeployment
		err := suite.k.Client.Get(context.Background(), key, &obj)
		if err == nil {
			return false
		}
		if !errors.IsNotFound(err) {
			return false
		}
		return true
	}, timeout, time.Second).Should(BeTrue())

	if delete {
		suite.Run("cm1 was deleted", func() {
			err := suite.k.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "cm1",
				Namespace: p.TestSlug(),
			}, cm)
			g.Expect(err).To(MatchError("configmaps \"cm1\" not found"))
		})
	} else {
		suite.Run("cm1 was not deleted", func() {
			err := suite.k.Client.Get(context.TODO(), client.ObjectKey{
				Name:      "cm1",
				Namespace: p.TestSlug(),
			}, cm)
			g.Expect(err).To(Succeed())
		})
	}
}

func (suite *GitOpsPruneDeleteSuite) Test_Delete_True() {
	suite.doTestDelete(true)
}

func (suite *GitOpsPruneDeleteSuite) Test_Delete_False() {
	suite.doTestDelete(false)
}
