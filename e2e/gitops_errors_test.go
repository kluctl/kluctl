package e2e

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/meta"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (suite *GitopsTestSuite) getReadiness(obj *kluctlv1.KluctlDeployment) *metav1.Condition {
	for _, c := range obj.Status.Conditions {
		if c.Type == meta.ReadyCondition {
			return &c
		}
	}
	return nil
}

func (suite *GitopsTestSuite) assertErrors(key client.ObjectKey, rstatus metav1.ConditionStatus, rreason string, rmessage string, expectedErrors []result.DeploymentError, expectedWarnings []result.DeploymentError) {
	g := NewWithT(suite.T())

	var kd kluctlv1.KluctlDeployment
	err := suite.k.Client.Get(context.TODO(), key, &kd)
	g.Expect(err).To(Succeed())

	g.Expect(kd.Status.LastDeployResult).ToNot(BeNil())

	readinessCondition := suite.getReadiness(&kd)
	g.Expect(readinessCondition).ToNot(BeNil())

	g.Expect(readinessCondition.Status).To(Equal(rstatus))
	g.Expect(readinessCondition.Reason).To(Equal(rreason))
	g.Expect(readinessCondition.Message).To(ContainSubstring(rmessage))

	rs, err := results.NewResultStoreSecrets(context.TODO(), suite.k.Client, "", 0)
	g.Expect(err).To(Succeed())

	lastDeployResult, err := kd.Status.GetLastDeployResult()
	g.Expect(err).To(Succeed())
	cr, err := rs.GetCommandResult(results.GetCommandResultOptions{
		Id: lastDeployResult.Id,
	})
	g.Expect(err).To(Succeed())

	g.Expect(cr.Errors).To(ConsistOf(expectedErrors))
	g.Expect(cr.Warnings).To(ConsistOf(expectedWarnings))

	g.Expect(lastDeployResult.Errors).To(ConsistOf(expectedErrors))
	g.Expect(lastDeployResult.Warnings).To(ConsistOf(expectedWarnings))
}

func (suite *GitopsTestSuite) TestGitOpsErrors() {
	g := NewWithT(suite.T())
	_ = g

	p := test_utils.NewTestProject(suite.T())
	createNamespace(suite.T(), suite.k, p.TestSlug())

	goodCm1 := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: "{{ args.namespace }}"
data:
  k1: v1
`
	badCm1_1 := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: "{{ args.namespace }}"
data_error:
  k1: v1
`
	badCm1_2 := `apiVersion: v1
kind: ConfigMap
metadata:
  name: cm1
  namespace: "{{ args.namespace }}"
data:
  k1: {
`

	p.UpdateTarget("target1", nil)
	p.AddKustomizeDeployment("d1", []test_utils.KustomizeResource{
		{Name: "cm1.yaml", Content: uo.FromStringMust(goodCm1)},
	}, nil)

	key := suite.createKluctlDeployment(p, "target1", map[string]any{
		"namespace": p.TestSlug(),
	})

	suite.Run("initial deployment", func() {
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})

	cm1Ref := k8s.NewObjectRef("", "v1", "ConfigMap", "cm1", p.TestSlug())

	suite.Run("cm1 causes error while applying", func() {
		p.UpdateFile("d1/cm1.yaml", func(f string) (string, error) {
			return badCm1_1, nil
		}, "")
		suite.waitForReconcile(key)
		suite.assertErrors(key, metav1.ConditionFalse, kluctlv1.DeployFailedReason, "deploy failed with 1 errors", []result.DeploymentError{
			{
				Ref:     cm1Ref,
				Message: "failed to patch test-git-ops-test-git-ops-errors/ConfigMap/cm1: failed to create typed patch object (test-git-ops-test-git-ops-errors/cm1; /v1, Kind=ConfigMap): .data_error: field not declared in schema",
			},
		}, nil)
		p.UpdateFile("d1/cm1.yaml", func(f string) (string, error) {
			return goodCm1, nil
		}, "")
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})

	suite.Run("cm1 causes error while loading", func() {
		p.UpdateFile("d1/cm1.yaml", func(f string) (string, error) {
			return badCm1_2, nil
		}, "")
		suite.waitForReconcile(key)
		suite.assertErrors(key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "MalformedYAMLError: yaml: line 7: did not find expected node content in File: cm1.yaml", nil, nil)
		p.UpdateFile("d1/cm1.yaml", func(f string) (string, error) {
			return goodCm1, nil
		}, "")
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})

	suite.Run("project can't be loaded", func() {
		kluctlBackup := ""
		p.UpdateFile(".kluctl.yml", func(f string) (string, error) {
			kluctlBackup = f
			return "a: b", nil
		}, "")
		suite.waitForReconcile(key)
		suite.assertErrors(key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, ".kluctl.yml failed: error unmarshaling JSON: while decoding JSON: json: unknown field \"a\"", nil, nil)
		p.UpdateFile(".kluctl.yml", func(f string) (string, error) {
			return kluctlBackup, nil
		}, "")
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})

	suite.Run("deployment can't be loaded", func() {
		deploymentBackup := ""
		p.UpdateFile("deployment.yml", func(f string) (string, error) {
			deploymentBackup = f
			return "a: b", nil
		}, "")
		suite.waitForReconcile(key)
		suite.assertErrors(key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "failed to load deployment.yml: error unmarshaling JSON: while decoding JSON: json: unknown field \"a\"", nil, nil)
		p.UpdateFile("deployment.yml", func(f string) (string, error) {
			return deploymentBackup, nil
		}, "")
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})

	suite.Run("invalid target", func() {
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Target = utils.StrPtr("invalid")
		})
		suite.waitForReconcile(key)
		suite.assertErrors(key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "target invalid not existent in kluctl project config", nil, nil)
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Target = utils.StrPtr("target1")
		})
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})

	suite.Run("invalid context", func() {
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Context = utils.StrPtr("invalid")
		})
		suite.waitForReconcile(key)
		suite.assertErrors(key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "context \"invalid\" does not exist", nil, nil)
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Context = utils.StrPtr("default")
		})
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})

	suite.Run("non existing git repo", func() {
		var backup types.GitUrl
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			backup = kd.Spec.Source.URL
			kd.Spec.Source.URL = *types.ParseGitUrlMust(backup.String() + "/invalid")
		})
		suite.waitForReconcile(key)
		suite.assertErrors(key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "failed clone source: repository not found", nil, nil)
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Source.URL = backup
		})
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})
	suite.Run("non existing git branch", func() {
		var backup *types.GitRef
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			backup = kd.Spec.Source.Ref
			kd.Spec.Source.Ref = &types.GitRef{
				Branch: "invalid",
			}
		})
		suite.waitForReconcile(key)
		suite.assertErrors(key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "ref refs/heads/invalid not found", nil, nil)
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Source.Ref = backup
		})
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})

	suite.Run("prune without discriminator", func() {
		var backup any
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Prune = true
		})
		p.UpdateKluctlYaml(func(o *uo.UnstructuredObject) error {
			backup, _, _ = o.GetNestedField("discriminator")
			_ = o.RemoveNestedField("discriminator")
			return nil
		})
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		suite.assertErrors(key, metav1.ConditionFalse, kluctlv1.DeployFailedReason, "deploy failed with 1 errors", []result.DeploymentError{
			{Message: "pruning without a discriminator is not supported"},
		}, nil)
		p.UpdateKluctlYaml(func(o *uo.UnstructuredObject) error {
			_ = o.SetNestedField(backup, "discriminator")
			return nil
		})
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		suite.assertErrors(key, metav1.ConditionTrue, kluctlv1.ReconciliationSucceededReason, "deploy: ok", nil, nil)
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Prune = false
		})
	})
}
