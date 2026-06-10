package e2e

import (
	"testing"

	"github.com/kluctl/kluctl/v2/api/v1beta1"
	git2 "github.com/kluctl/kluctl/v2/e2e/test-utils"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type GitOpsIncludesSuite struct {
	GitopsTestSuite
}

func TestGitOpsIncludes(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GitOpsIncludesSuite))
}

func (suite *GitOpsIncludesSuite) testGitOpsGitIncludeCredentials() {
	g := NewWithT(suite.T())
	_ = g

	mainGs := git2.NewTestGitServer(suite.T(), git2.WithTestGitServerAuth("user1", "password1"))
	gs1 := git2.NewTestGitServer(suite.T(), git2.WithTestGitServerAuth("user2", "password2"))
	gs2 := git2.NewTestGitServer(suite.T())

	p, ip1, _ := prepareGitIncludeTest(suite.T(), suite.k, mainGs, gs1, gs2)

	key := suite.createKluctlDeployment(p, "test", nil)

	suite.Run("fail without authentication", func() {
		suite.waitForCommit(key, "")

		kd := suite.getKluctlDeployment(key)

		readinessCondition := suite.getReadiness(kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionFalse))
		g.Expect(readinessCondition.Reason).To(Equal("PrepareFailed"))
		g.Expect(kd.Status.LastPrepareError).To(ContainSubstring("failed to clone git source: http transport: authentication required"))
	})

	createSecret := func(username string, password string) string {
		return suite.createGitopsSecret(map[string]string{
			"username": username,
			"password": password,
		})
	}

	secret1 := createSecret("user1", "password1")
	secret2 := createSecret("user2", "password2")

	suite.updateKluctlDeployment(key, func(kd *v1beta1.KluctlDeployment) {
		var credentials []v1beta1.ProjectCredentialsGit
		credentials = append(credentials, v1beta1.ProjectCredentialsGit{
			Host:      gs1.GitHost(),
			Path:      ip1.GitUrlPath(),
			SecretRef: v1beta1.LocalObjectReference{Name: secret2},
		})
		credentials = append(credentials, v1beta1.ProjectCredentialsGit{
			Host:      mainGs.GitHost(),
			SecretRef: v1beta1.LocalObjectReference{Name: secret1},
		})
		// make sure this one is ignored for http based urls
		credentials = append(credentials, v1beta1.ProjectCredentialsGit{
			Host:      "*",
			SecretRef: v1beta1.LocalObjectReference{Name: secret1},
		})
		kd.Spec.Credentials.Git = credentials
	})

	suite.Run("retry with authentication", func() {
		kd := suite.waitForCommit(key, getHeadRevision(suite.T(), p))

		readinessCondition := suite.getReadiness(kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionTrue))
	})
}

func (suite *GitOpsIncludesSuite) TestGitOpsGitIncludeCredentials() {
	suite.testGitOpsGitIncludeCredentials()
}
