package e2e

import (
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/stretchr/testify/suite"
	"testing"
)

type GitOpsApprovalTestSuite struct {
	GitopsTestSuite
}

func TestGitOpsApproval(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GitOpsApprovalTestSuite))
}

func (suite *GitOpsApprovalTestSuite) TestGitOpsManualDeployment() {
	p := test_project.NewTestProject(suite.T())
	createNamespace(suite.T(), suite.k, p.TestSlug())

	p.UpdateTarget("target1", nil)
	addConfigMapDeployment(p, "d1", nil, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})

	key := suite.createKluctlDeployment(p, "target1", nil)

	suite.Run("initial deployment", func() {
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm1")
	})

	suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.Manual = true
	})
	suite.waitForReconcile(key)

	addConfigMapDeployment(p, "d2", nil, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
	})

	suite.Run("manual deployment not triggered", func() {
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		assertConfigMapNotExists(suite.T(), suite.k, p.TestSlug(), "cm2")
	})

	suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.ManualObjectsHash = &kd.Status.LastObjectsHash
	})

	suite.Run("manual deployment triggered", func() {
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm2")
	})
}
