package e2e

import (
	"fmt"
	gittypes "github.com/kluctl/kluctl/lib/git/types"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strings"
	"testing"
)

type GitOpsLocalSourceOverrideSuite struct {
	GitopsTestSuite
}

func TestGitOpsLocalSourceOverride(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GitOpsLocalSourceOverrideSuite))
}

func (suite *GitOpsLocalSourceOverrideSuite) assertOverridesDidNotHappen(key client.ObjectKey, pt *preparedSourceOverrideTest) {
	cm := assertConfigMapExists(suite.T(), suite.k, pt.p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(suite.T(), cm, "v", "data", "a")
	cm = assertConfigMapExists(suite.T(), suite.k, pt.p.TestSlug(), "include2-cm")
	assertNestedFieldEquals(suite.T(), cm, "v", "data", "a")
}

func (suite *GitOpsLocalSourceOverrideSuite) assertOverridesDidHappen(key client.ObjectKey, pt *preparedSourceOverrideTest) {
	kd := suite.getKluctlDeployment(key)
	assert.NotNil(suite.T(), kd.Status.DeployRequestResult)
	assert.NotEmpty(suite.T(), kd.Status.DeployRequestResult.ResultId)

	suite.assertChanges(kd.Status.DeployRequestResult.ResultId, 0, 2, 0, 0)
	cm := assertConfigMapExists(suite.T(), suite.k, pt.p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(suite.T(), cm, "o1", "data", "a")
	cm = assertConfigMapExists(suite.T(), suite.k, pt.p.TestSlug(), "include2-cm")
	assertNestedFieldEquals(suite.T(), cm, "o2", "data", "a")
}

func (suite *GitOpsLocalSourceOverrideSuite) TestLocalGitOverrides() {
	gs := test_utils.NewTestGitServer(suite.T())
	pt := prepareLocalSourceOverrideTest(suite.T(), suite.k, gs, false)
	pt.p.AddExtraArgs("--controller-namespace", suite.gitopsNamespace+"-system")

	key := suite.createKluctlDeployment(pt.p, "test", nil)

	suite.Run("initial deployment", func() {
		pt.p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)
		suite.assertOverridesDidNotHappen(key, &pt)
	})

	suite.Run("suspending the deployment", func() {
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Suspend = true
		})
		suite.waitForReconcile(key)
	})

	suite.Run("deploy with overridden git source", func() {
		u1, _ := gittypes.ParseGitUrl(pt.ip1.GitUrl())
		u2, _ := gittypes.ParseGitUrl(pt.ip2.GitUrl())
		k1 := u1.RepoKey().String()
		k2 := u2.RepoKey().String()

		suite.assertOverridesDidNotHappen(key, &pt)

		pt.p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name,
			"--local-git-override", fmt.Sprintf("%s=%s", k1, pt.override1),
			"--local-git-override", fmt.Sprintf("%s=%s", k2, pt.override2),
			"--local-source-override-port", fmt.Sprintf("%d", suite.sourceOverridePort))

		suite.assertOverridesDidHappen(key, &pt)

		// undo everything
		pt.p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)
	})

	suite.Run("deploy with overridden git group source", func() {
		u1, _ := gittypes.ParseGitUrl(pt.p.GitServer().GitUrl() + "/repos")
		k1 := u1.RepoKey().String()

		suite.assertOverridesDidNotHappen(key, &pt)

		pt.p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name,
			"--local-git-group-override", fmt.Sprintf("%s=%s", k1, pt.overrideGroupDir),
			"--local-source-override-port", fmt.Sprintf("%d", suite.sourceOverridePort))

		suite.assertOverridesDidHappen(key, &pt)

		// undo everything
		pt.p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)
	})
}

func (suite *GitOpsLocalSourceOverrideSuite) TestLocalOciOverrides() {
	pt := prepareLocalSourceOverrideTest(suite.T(), suite.k, nil, true)
	pt.p.AddExtraArgs("--controller-namespace", suite.gitopsNamespace+"-system")

	key := suite.createKluctlDeployment(pt.p, "test", nil)

	suite.Run("initial deployment", func() {
		pt.p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)
		suite.assertOverridesDidNotHappen(key, &pt)
	})

	suite.Run("suspending the deployment", func() {
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Suspend = true
		})
		suite.waitForReconcile(key)
	})

	suite.Run("deploy with overridden oci source", func() {
		k1 := strings.TrimPrefix(pt.repoUrl1, "oci://")
		k2 := strings.TrimPrefix(pt.repoUrl2, "oci://")

		suite.assertOverridesDidNotHappen(key, &pt)

		pt.p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name,
			"--local-oci-override", fmt.Sprintf("%s=%s", k1, pt.override1),
			"--local-oci-override", fmt.Sprintf("%s=%s", k2, pt.override2),
			"--local-source-override-port", fmt.Sprintf("%d", suite.sourceOverridePort))

		suite.assertOverridesDidHappen(key, &pt)

		// undo everything
		pt.p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)
	})

	suite.Run("deploy with overridden oci group source", func() {
		k1 := strings.TrimPrefix(pt.repo.URL.String(), "oci://") + "/org1"

		suite.assertOverridesDidNotHappen(key, &pt)

		pt.p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name,
			"--local-oci-group-override", fmt.Sprintf("%s=%s", k1, pt.overrideGroupDir),
			"--local-source-override-port", fmt.Sprintf("%d", suite.sourceOverridePort))

		suite.assertOverridesDidHappen(key, &pt)

		// undo everything
		pt.p.KluctlMust(suite.T(), "gitops", "deploy", "--context", suite.k.Context, "--namespace", key.Namespace, "--name", key.Name)
	})
}
