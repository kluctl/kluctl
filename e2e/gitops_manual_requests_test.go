package e2e

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

type GitOpsManualRequestsSuite struct {
	GitopsTestSuite
}

func TestGitOpsManualRequests(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GitOpsManualRequestsSuite))
}

func (suite *GitOpsManualRequestsSuite) TestManualRequests() {
	g := NewWithT(suite.T())

	p := test_project.NewTestProject(suite.T())
	createNamespace(suite.T(), suite.k, p.TestSlug())

	p.UpdateTarget("target1", nil)
	addConfigMapDeployment(p, "d1", nil, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})
	p.UpdateYaml("d1/configmap-cm1.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("v1", "data", "k")
		return nil
	}, "")

	key := suite.createKluctlDeployment(p, "target1", nil)

	suite.Run("initial deployment", func() {
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
	})

	suite.Run("suspending the deployment", func() {
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Suspend = true
			// this will get important later when we test suspend/resume
			kd.Spec.DeployInterval = &kluctlv1.SafeDuration{Duration: metav1.Duration{Duration: time.Second * 2}}
		})
		suite.waitForReconcile(key)
	})

	suite.Run("run manual diff (with no changes)", func() {
		p.KluctlMust(suite.T(), "gitops", "diff", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.DiffRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.DiffRequestResult.ResultId)

		suite.assertNoChanges(kd.Status.DiffRequestResult.ResultId)
	})

	p.UpdateYaml("d1/configmap-cm1.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("v2", "data", "k")
		return nil
	}, "")
	addConfigMapDeployment(p, "d2", nil, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
	})

	suite.Run("run manual diff (with changes)", func() {
		p.KluctlMust(suite.T(), "gitops", "diff", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.DiffRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.DiffRequestResult.ResultId)

		suite.assertChanges(kd.Status.DiffRequestResult.ResultId, 1, 1, 0, 0)

		// assert nothing actually changed
		cm1 := assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
		assertNestedFieldEquals(suite.T(), cm1, "v1", "data", "k")
		assertConfigMapNotExists(suite.T(), suite.k, p.TestSlug(), "cm2")
	})

	suite.Run("run manual deploy (with changes)", func() {
		p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.DeployRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.DeployRequestResult.ResultId)

		suite.assertChanges(kd.Status.DeployRequestResult.ResultId, 1, 1, 0, 0)

		// assert it actually changed
		cm1 := assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
		assertNestedFieldEquals(suite.T(), cm1, "v2", "data", "k")
		assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm2")
	})

	suite.Run("run manual deploy (with no changes)", func() {
		p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.DeployRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.DeployRequestResult.ResultId)

		suite.assertNoChanges(kd.Status.DeployRequestResult.ResultId)

		// assert nothing actually changed
		cm1 := assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
		assertNestedFieldEquals(suite.T(), cm1, "v2", "data", "k")
		assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm2")
	})

	suite.Run("run manual prune (with no changes)", func() {
		p.KluctlMust(suite.T(), "gitops", "prune", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.PruneRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.PruneRequestResult.ResultId)

		suite.assertNoChanges(kd.Status.PruneRequestResult.ResultId)
	})

	p.DeleteKustomizeDeployment("d2")

	suite.Run("run manual prune (with changes)", func() {
		p.KluctlMust(suite.T(), "gitops", "prune", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.PruneRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.PruneRequestResult.ResultId)

		suite.assertChanges(kd.Status.PruneRequestResult.ResultId, 0, 0, 0, 1)
		assertConfigMapNotExists(suite.T(), suite.k, p.TestSlug(), "cm2")
	})

	suite.Run("run manual validate (with no errors)", func() {
		p.KluctlMust(suite.T(), "gitops", "validate", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.ValidateRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.ValidateRequestResult.ResultId)

		vr := suite.getValidateResult(kd.Status.ValidateRequestResult.ResultId)
		assert.Empty(suite.T(), vr.Errors)
		assert.Empty(suite.T(), vr.Warnings)
	})

	cm1 := assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
	err := suite.k.Client.Delete(context.Background(), cm1.ToUnstructured())
	assert.NoError(suite.T(), err)

	suite.Run("run manual validate (with errors)", func() {
		_, _, err := p.Kluctl(suite.T(), "gitops", "validate", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)
		assert.ErrorContains(suite.T(), err, "Validation failed")

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.ValidateRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.ValidateRequestResult.ResultId)

		vr := suite.getValidateResult(kd.Status.ValidateRequestResult.ResultId)
		assert.Equal(suite.T(), []result.DeploymentError{{Ref: k8s.ObjectRef{Group: "", Version: "v1", Kind: "ConfigMap", Name: "cm1", Namespace: p.TestSlug()}, Message: "object not found"}}, vr.Errors)
		assert.Empty(suite.T(), vr.Warnings)
	})

	suite.Run("resume and wait for reconcile", func() {
		p.KluctlMust(suite.T(), "gitops", "resume", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)
		g.Eventually(func() bool {
			var cm1 corev1.ConfigMap
			err := suite.k.Client.Get(context.Background(), client.ObjectKey{Name: "cm1", Namespace: p.TestSlug()}, &cm1)
			return err == nil
		}, timeout, time.Second).Should(BeTrue())

		// delete it again and ensure it re-appears (we have deployInterval=2s)
		err := suite.k.Client.Delete(context.Background(), cm1.ToUnstructured())
		assert.NoError(suite.T(), err)

		g.Eventually(func() bool {
			var cm1 corev1.ConfigMap
			err := suite.k.Client.Get(context.Background(), client.ObjectKey{Name: "cm1", Namespace: p.TestSlug()}, &cm1)
			return err == nil
		}, timeout, time.Second).Should(BeTrue())
	})

	suite.Run("suspend and ensure reconcile does not happen", func() {
		p.KluctlMust(suite.T(), "gitops", "suspend", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)

		err := suite.k.Client.Delete(context.Background(), cm1.ToUnstructured())
		assert.NoError(suite.T(), err)

		g.Consistently(func() bool {
			var cm1 corev1.ConfigMap
			err := suite.k.Client.Get(context.Background(), client.ObjectKey{Name: "cm1", Namespace: p.TestSlug()}, &cm1)
			return errors.IsNotFound(err)
		}, 5*time.Second, time.Second).Should(BeTrue())

		// ensure source is changed
		p.UpdateYaml("d1/configmap-cm1.yml", func(o *uo.UnstructuredObject) error {
			_ = o.SetNestedField("v3", "data", "k")
			return nil
		}, "")

		// and it should still not reconcile
		g.Consistently(func() bool {
			var cm1 corev1.ConfigMap
			err := suite.k.Client.Get(context.Background(), client.ObjectKey{Name: "cm1", Namespace: p.TestSlug()}, &cm1)
			return errors.IsNotFound(err)
		}, 5*time.Second, time.Second).Should(BeTrue())
	})

	suite.Run("run manual reconcile", func() {
		assertConfigMapNotExists(suite.T(), suite.k, p.TestSlug(), "cm1")

		// this should run even though suspend=true
		p.KluctlMust(suite.T(), "gitops", "reconcile", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.ReconcileRequestResult)

		assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
	})
}

func (suite *GitOpsManualRequestsSuite) TestOverrides() {
	p := test_project.NewTestProject(suite.T())
	createNamespace(suite.T(), suite.k, p.TestSlug())

	p.UpdateTarget("target1", nil)
	addConfigMapDeployment(p, "d1", nil, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})
	p.UpdateYaml("d1/configmap-cm1.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("v1", "data", "k1")
		_ = o.SetNestedField("{{ args.a }}", "data", "k2")
		_ = o.SetNestedField(`{{ get_var("args.b", "na") }}`, "data", "k3")
		return nil
	}, "")

	key := suite.createKluctlDeployment(p, "target1", map[string]any{
		"a": "v1",
	})

	suite.Run("initial deployment", func() {
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		cm1 := assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
		assertNestedFieldEquals(suite.T(), cm1, "v1", "data", "k1")
		assertNestedFieldEquals(suite.T(), cm1, "v1", "data", "k2")
		assertNestedFieldEquals(suite.T(), cm1, "na", "data", "k3")
	})

	suite.Run("suspending the deployment", func() {
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Suspend = true
		})
		suite.waitForReconcile(key)
	})

	p.UpdateYaml("d1/configmap-cm1.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("v2", "data", "k1")
		return nil
	}, "")
	addConfigMapDeployment(p, "d2", nil, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
	})

	suite.Run("deploy with dry-run", func() {
		p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name, "--dry-run")

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.DeployRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.DeployRequestResult.ResultId)

		// assert that the result pretends that it was changed
		suite.assertChanges(kd.Status.DeployRequestResult.ResultId, 1, 1, 0, 0)

		// assert that in reality nothing was changed
		cm1 := assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
		assertNestedFieldEquals(suite.T(), cm1, "v1", "data", "k1")
		assertConfigMapNotExists(suite.T(), suite.k, p.TestSlug(), "cm2")

		// now re-deploy with dry-run=false
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.DryRun = true
		})
		suite.waitForReconcile(key)
		p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name, "--dry-run=false")

		kd = suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.DeployRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.DeployRequestResult.ResultId)

		suite.assertChanges(kd.Status.DeployRequestResult.ResultId, 1, 1, 0, 0)

		// assert it actually changed this time
		cm1 = assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
		assertNestedFieldEquals(suite.T(), cm1, "v2", "data", "k1")
		assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm2")
	})

	// we've set dryRun=true in the previous test, let's undo this
	suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.DryRun = false
	})
	suite.waitForReconcile(key)

	suite.Run("deploy with overridden args", func() {
		p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name, "-a", "a=via_arg")

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.DeployRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.DeployRequestResult.ResultId)

		suite.assertChanges(kd.Status.DeployRequestResult.ResultId, 0, 1, 0, 0)

		// assert it actually changed this time
		cm1 := assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
		assertNestedFieldEquals(suite.T(), cm1, "via_arg", "data", "k2")

		// undo it
		p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)
	})

	p.DeleteKustomizeDeployment("d2")

	suite.Run("deploy with overridden prune", func() {
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Prune = true
		})

		p.KluctlMust(suite.T(), "gitops", "reconcile", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name, "--prune=false")
		p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name, "--prune=false")

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.DeployRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.DeployRequestResult.ResultId)

		suite.assertChanges(kd.Status.DeployRequestResult.ResultId, 0, 0, 1, 0)
		assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm2")

		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Prune = false
		})
		p.KluctlMust(suite.T(), "gitops", "reconcile", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)
		p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name, "--prune")

		kd = suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.DeployRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.DeployRequestResult.ResultId)

		suite.assertChanges(kd.Status.DeployRequestResult.ResultId, 0, 0, 0, 1)
		assertConfigMapNotExists(suite.T(), suite.k, p.TestSlug(), "cm2")
	})

	p.UpdateYaml("d1/configmap-cm1.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("v1", "data", "k4")
		return nil
	}, "")
	addConfigMapDeployment(p, "d3", nil, resourceOpts{
		name:      "cm3",
		namespace: p.TestSlug(),
	})

	suite.Run("deploy with overridden inclusion", func() {
		p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name, "-I", "d1")

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.DeployRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.DeployRequestResult.ResultId)

		suite.assertChanges(kd.Status.DeployRequestResult.ResultId, 0, 1, 0, 0)
		assertConfigMapNotExists(suite.T(), suite.k, p.TestSlug(), "cm3")
	})

	suite.Run("deploy with overridden exclusion", func() {
		p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name, "-E", "d1")

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.DeployRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.DeployRequestResult.ResultId)

		suite.assertChanges(kd.Status.DeployRequestResult.ResultId, 1, 0, 0, 0)
		assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm3")
	})

	p.UpdateTarget("target2", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField("via_target", "args", "b")
	})

	suite.Run("deploy with overridden target", func() {
		// first, deploy without overrides
		p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)
		cm1 := assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
		assertNestedFieldEquals(suite.T(), cm1, "na", "data", "k3")

		// now with override
		p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name, "--target", "target2")

		kd := suite.getKluctlDeployment(key)
		assert.NotNil(suite.T(), kd.Status.DeployRequestResult)
		assert.NotEmpty(suite.T(), kd.Status.DeployRequestResult.ResultId)

		suite.assertChanges(kd.Status.DeployRequestResult.ResultId, 0, 2, 0, 0)
		cm1 = assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
		cm3 := assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm3")
		assertNestedFieldEquals(suite.T(), cm1, p.TestSlug()+"-target2", "metadata", "labels", "kluctl.io/discriminator")
		assertNestedFieldEquals(suite.T(), cm3, p.TestSlug()+"-target2", "metadata", "labels", "kluctl.io/discriminator")
		assertNestedFieldEquals(suite.T(), cm1, "via_target", "data", "k3")
	})
}
