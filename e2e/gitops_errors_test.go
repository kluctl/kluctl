package e2e

import (
	gittypes "github.com/kluctl/kluctl/lib/git/types"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

type GitOpsErrorsSuite struct {
	GitopsTestSuite
}

func TestGitOpsErrors(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GitOpsErrorsSuite))
}

func (suite *GitOpsErrorsSuite) assertErrors(kd *kluctlv1.KluctlDeployment, rstatus metav1.ConditionStatus, rreason string, rmessage string, prepareError string, expectedErrors []result.DeploymentError, expectedWarnings []result.DeploymentError) {
	g := NewWithT(suite.T())

	g.Expect(kd.Status.LastDeployResult).ToNot(BeNil())

	readinessCondition := suite.getReadiness(kd)
	g.Expect(readinessCondition).ToNot(BeNil())

	g.Expect(readinessCondition.Status).To(Equal(rstatus))
	g.Expect(readinessCondition.Reason).To(Equal(rreason))
	g.Expect(readinessCondition.Message).To(ContainSubstring(rmessage))

	g.Expect(kd.Status.LastPrepareError, prepareError)

	lastDeployResult, err := kd.Status.GetLastDeployResult()
	g.Expect(err).To(Succeed())

	g.Expect(err).To(Succeed())
	if len(expectedErrors) != 0 || len(expectedWarnings) != 0 {
		cr := suite.getCommandResult(lastDeployResult.Id)
		g.Expect(cr).ToNot(BeNil())
		g.Expect(err).To(Succeed())
		g.Expect(cr.Errors).To(ConsistOf(expectedErrors))
		g.Expect(cr.Warnings).To(ConsistOf(expectedWarnings))
	}

	g.Expect(lastDeployResult.Errors).To(ConsistOf(expectedErrors))
	g.Expect(lastDeployResult.Warnings).To(ConsistOf(expectedWarnings))
}

func (suite *GitOpsErrorsSuite) TestGitOpsErrors() {
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
		kd := suite.waitForReconcile(key)
		suite.assertErrors(kd, metav1.ConditionFalse, kluctlv1.DeployFailedReason, "deploy failed with 1 errors", "", []result.DeploymentError{
			{
				Ref:     cm1Ref,
				Message: "failed to patch git-ops-errors-git-ops-errors/ConfigMap/cm1: failed to create typed patch object (git-ops-errors-git-ops-errors/cm1; /v1, Kind=ConfigMap): .data_error: field not declared in schema",
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
		kd := suite.waitForReconcile(key)
		suite.assertErrors(kd, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "prepare failed with 1 errors. Check status.lastPrepareError for details", "MalformedYAMLError: yaml: line 7: did not find expected node content in File: cm1.yaml", nil, nil)
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
		kd := suite.waitForReconcile(key)
		suite.assertErrors(kd, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "prepare failed. Check status.lastPrepareError for details", ".kluctl.yml failed: error unmarshaling JSON: while decoding JSON: json: unknown field \"a\"", nil, nil)
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
		kd := suite.waitForReconcile(key)
		suite.assertErrors(kd, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "prepare failed. Check status.lastPrepareError for details", "failed to load deployment.yml: error unmarshaling JSON: while decoding JSON: json: unknown field \"a\"", nil, nil)
		p.UpdateFile("deployment.yml", func(f string) (string, error) {
			return deploymentBackup, nil
		}, "")
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})

	suite.Run("invalid target", func() {
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Target = utils.Ptr("invalid")
		})
		kd := suite.waitForReconcile(key)
		suite.assertErrors(kd, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "prepare failed. Check status.lastPrepareError for details", "target invalid not existent in kluctl project config", nil, nil)
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Target = utils.Ptr("target1")
		})
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})

	suite.Run("invalid context", func() {
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Context = utils.Ptr("invalid")
		})
		kd := suite.waitForReconcile(key)
		suite.assertErrors(kd, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "prepare failed. Check status.lastPrepareError for details", "context \"invalid\" does not exist", nil, nil)
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Context = utils.Ptr("default")
		})
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})

	suite.Run("non existing git repo", func() {
		var backup string
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			backup = kd.Spec.Source.Git.URL
			kd.Spec.Source.Git.URL = backup + "/invalid"
		})
		kd := suite.waitForReconcile(key)
		suite.assertErrors(kd, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "prepare failed. Check status.lastPrepareError for details", "failed to clone git source: repository not found", nil, nil)
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Source.Git.URL = backup
		})
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
	})
	suite.Run("non existing git branch", func() {
		var backup *gittypes.GitRef
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			backup = kd.Spec.Source.Git.Ref
			kd.Spec.Source.Git.Ref = &gittypes.GitRef{
				Branch: "invalid",
			}
		})
		kd := suite.waitForReconcile(key)
		suite.assertErrors(kd, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "prepare failed. Check status.lastPrepareError for details", "ref refs/heads/invalid not found", nil, nil)
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Source.Git.Ref = backup
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
		kd := suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		suite.assertErrors(kd, metav1.ConditionFalse, kluctlv1.DeployFailedReason, "deploy failed with 1 errors", "", []result.DeploymentError{
			{Message: "pruning without a discriminator is not supported"},
		}, []result.DeploymentError{
			{Message: "no discriminator configured. Orphan object detection will not work"},
		})
		p.UpdateKluctlYaml(func(o *uo.UnstructuredObject) error {
			_ = o.SetNestedField(backup, "discriminator")
			return nil
		})
		kd = suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		suite.assertErrors(kd, metav1.ConditionTrue, kluctlv1.ReconciliationSucceededReason, "deploy succeeded", "", nil, nil)
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Prune = false
		})
	})
}
