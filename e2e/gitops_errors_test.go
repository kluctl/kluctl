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
	"testing"
)

func getReadiness(obj *kluctlv1.KluctlDeployment) *metav1.Condition {
	for _, c := range obj.Status.Conditions {
		if c.Type == meta.ReadyCondition {
			return &c
		}
	}
	return nil
}

func assertErrors(t *testing.T, k *test_utils.EnvTestCluster, key client.ObjectKey, rstatus metav1.ConditionStatus, rreason string, rmessage string, expectedErrors []result.DeploymentError, expectedWarnings []result.DeploymentError) {
	g := NewWithT(t)

	var kd kluctlv1.KluctlDeployment
	err := k.Client.Get(context.TODO(), key, &kd)
	g.Expect(err).To(Succeed())

	g.Expect(kd.Status.LastDeployResult).ToNot(BeNil())

	readinessCondition := getReadiness(&kd)
	g.Expect(readinessCondition).ToNot(BeNil())

	g.Expect(readinessCondition.Status).To(Equal(rstatus))
	g.Expect(readinessCondition.Reason).To(Equal(rreason))
	g.Expect(readinessCondition.Message).To(ContainSubstring(rmessage))

	rs, err := results.NewResultStoreSecrets(context.TODO(), k.Client, nil, "", 0)
	g.Expect(err).To(Succeed())

	cr, err := rs.GetCommandResult(results.GetCommandResultOptions{
		Id: kd.Status.LastDeployResult.Id,
	})
	g.Expect(err).To(Succeed())

	g.Expect(cr.Errors).To(ConsistOf(expectedErrors))
	g.Expect(cr.Warnings).To(ConsistOf(expectedWarnings))

	g.Expect(kd.Status.LastDeployResult.Errors).To(Equal(len(expectedErrors)))
	g.Expect(kd.Status.LastDeployResult.Warnings).To(Equal(len(expectedWarnings)))
}

func TestGitOpsErrors(t *testing.T) {
	g := NewWithT(t)
	_ = g

	startKluctlController(t)

	k := defaultCluster1

	p := test_utils.NewTestProject(t)
	createNamespace(t, k, p.TestSlug())

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

	key := createKluctlDeployment(t, p, k, "target1", map[string]any{
		"namespace": p.TestSlug(),
	})

	t.Run("initial deployment", func(t *testing.T) {
		waitForCommit(t, k, key, getHeadRevision(t, p))
	})

	cm1Ref := k8s.NewObjectRef("", "v1", "ConfigMap", "cm1", p.TestSlug())

	t.Run("cm1 causes error while applying", func(t *testing.T) {
		p.UpdateFile("d1/cm1.yaml", func(f string) (string, error) {
			return badCm1_1, nil
		}, "")
		waitForReconcile(t, k, key)
		assertErrors(t, k, key, metav1.ConditionFalse, kluctlv1.DeployFailedReason, "deploy failed with 1 errors", []result.DeploymentError{
			{
				Ref:     cm1Ref,
				Message: "failed to patch test-git-ops-errors/ConfigMap/cm1: failed to create typed patch object (test-git-ops-errors/cm1; /v1, Kind=ConfigMap): .data_error: field not declared in schema",
			},
		}, nil)
		p.UpdateFile("d1/cm1.yaml", func(f string) (string, error) {
			return goodCm1, nil
		}, "")
		waitForCommit(t, k, key, getHeadRevision(t, p))
	})

	t.Run("cm1 causes error while loading", func(t *testing.T) {
		p.UpdateFile("d1/cm1.yaml", func(f string) (string, error) {
			return badCm1_2, nil
		}, "")
		waitForReconcile(t, k, key)
		assertErrors(t, k, key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "MalformedYAMLError: yaml: line 7: did not find expected node content in File: cm1.yaml", nil, nil)
		p.UpdateFile("d1/cm1.yaml", func(f string) (string, error) {
			return goodCm1, nil
		}, "")
		waitForCommit(t, k, key, getHeadRevision(t, p))
	})

	t.Run("project can't be loaded", func(t *testing.T) {
		kluctlBackup := ""
		p.UpdateFile(".kluctl.yml", func(f string) (string, error) {
			kluctlBackup = f
			return "a: b", nil
		}, "")
		waitForReconcile(t, k, key)
		assertErrors(t, k, key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, ".kluctl.yml failed: error unmarshaling JSON: while decoding JSON: json: unknown field \"a\"", nil, nil)
		p.UpdateFile(".kluctl.yml", func(f string) (string, error) {
			return kluctlBackup, nil
		}, "")
		waitForCommit(t, k, key, getHeadRevision(t, p))
	})

	t.Run("deployment can't be loaded", func(t *testing.T) {
		deploymentBackup := ""
		p.UpdateFile("deployment.yml", func(f string) (string, error) {
			deploymentBackup = f
			return "a: b", nil
		}, "")
		waitForReconcile(t, k, key)
		assertErrors(t, k, key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "failed to load deployment.yml: error unmarshaling JSON: while decoding JSON: json: unknown field \"a\"", nil, nil)
		p.UpdateFile("deployment.yml", func(f string) (string, error) {
			return deploymentBackup, nil
		}, "")
		waitForCommit(t, k, key, getHeadRevision(t, p))
	})

	t.Run("invalid target", func(t *testing.T) {
		updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Target = utils.StrPtr("invalid")
		})
		waitForReconcile(t, k, key)
		assertErrors(t, k, key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "target invalid not existent in kluctl project config", nil, nil)
		updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Target = utils.StrPtr("target1")
		})
		waitForCommit(t, k, key, getHeadRevision(t, p))
	})

	t.Run("invalid context", func(t *testing.T) {
		updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Context = utils.StrPtr("invalid")
		})
		waitForReconcile(t, k, key)
		assertErrors(t, k, key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "context \"invalid\" does not exist", nil, nil)
		updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Context = utils.StrPtr("default")
		})
		waitForCommit(t, k, key, getHeadRevision(t, p))
	})

	t.Run("non existing git repo", func(t *testing.T) {
		var backup types.GitUrl
		updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
			backup = kd.Spec.Source.URL
			kd.Spec.Source.URL = *types.ParseGitUrlMust(backup.String() + "/invalid")
		})
		waitForReconcile(t, k, key)
		assertErrors(t, k, key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "failed clone source: repository not found", nil, nil)
		updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Source.URL = backup
		})
		waitForCommit(t, k, key, getHeadRevision(t, p))
	})
	t.Run("non existing git branch", func(t *testing.T) {
		var backup *kluctlv1.GitRef
		updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
			backup = kd.Spec.Source.Ref
			kd.Spec.Source.Ref = &kluctlv1.GitRef{
				Branch: "invalid",
			}
		})
		waitForReconcile(t, k, key)
		assertErrors(t, k, key, metav1.ConditionFalse, kluctlv1.PrepareFailedReason, "ref refs/heads/invalid not found", nil, nil)
		updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Source.Ref = backup
		})
		waitForCommit(t, k, key, getHeadRevision(t, p))
	})

	t.Run("prune without discriminator", func(t *testing.T) {
		var backup any
		updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Prune = true
		})
		p.UpdateKluctlYaml(func(o *uo.UnstructuredObject) error {
			backup, _, _ = o.GetNestedField("discriminator")
			_ = o.RemoveNestedField("discriminator")
			return nil
		})
		waitForCommit(t, k, key, getHeadRevision(t, p))
		assertErrors(t, k, key, metav1.ConditionFalse, kluctlv1.PruneFailedReason, "pruning without a discriminator is not supported", nil, nil)
		p.UpdateKluctlYaml(func(o *uo.UnstructuredObject) error {
			_ = o.SetNestedField(backup, "discriminator")
			return nil
		})
		waitForCommit(t, k, key, getHeadRevision(t, p))
		assertErrors(t, k, key, metav1.ConditionTrue, kluctlv1.ReconciliationSucceededReason, "deploy: ok, prune: ok", nil, nil)
		updateKluctlDeployment(t, k, key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Prune = false
		})
	})
}
