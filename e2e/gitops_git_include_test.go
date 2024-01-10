package e2e

import (
	"context"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	git2 "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

type GitOpsIncludesSuite struct {
	GitopsTestSuite
}

func TestGitOpsIncludes(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GitOpsIncludesSuite))
}

func (suite *GitOpsIncludesSuite) TestGitOpsGitIncludeDeprecatedSecret() {
	g := NewWithT(suite.T())
	_ = g

	mainGs := git2.NewTestGitServer(suite.T(), git2.WithTestGitServerAuth("user1", "password1"))
	gs1 := git2.NewTestGitServer(suite.T(), git2.WithTestGitServerAuth("user1", "password1"))
	gs2 := git2.NewTestGitServer(suite.T())

	p, _, _ := prepareGitIncludeTest(suite.T(), suite.k, mainGs, gs1, gs2)

	key := suite.createKluctlDeployment2(p, "test", nil, func(kd *v1beta1.KluctlDeployment) {
		kd.Spec.Source.URL = utils.StrPtr(p.GitUrl())
	})

	suite.Run("fail without authentication", func() {
		suite.waitForCommit(key, "")

		kd := suite.getKluctlDeployment(key)

		readinessCondition := suite.getReadiness(kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionFalse))
		g.Expect(readinessCondition.Reason).To(Equal("PrepareFailed"))
		g.Expect(readinessCondition.Message).To(Equal("failed to clone git source: authentication required"))
	})

	secret := corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "git-secrets",
			Namespace: key.Namespace,
		},
		Data: map[string][]byte{
			"username": []byte("user1"),
			"password": []byte("password1"),
		},
	}
	err := suite.k.Client.Create(context.Background(), &secret)
	g.Expect(err).To(Succeed())

	suite.updateKluctlDeployment(key, func(kd *v1beta1.KluctlDeployment) {
		kd.Spec.Source.SecretRef = &v1beta1.LocalObjectReference{Name: "git-secrets"}
	})

	suite.Run("retry with authentication", func() {
		kd := suite.waitForCommit(key, getHeadRevision(suite.T(), p))

		readinessCondition := suite.getReadiness(kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionTrue))
	})
}

func (suite *GitOpsIncludesSuite) testGitOpsGitIncludeCredentials(legacyGitSource bool) {
	g := NewWithT(suite.T())
	_ = g

	mainGs := git2.NewTestGitServer(suite.T(), git2.WithTestGitServerAuth("user1", "password1"))
	gs1 := git2.NewTestGitServer(suite.T(), git2.WithTestGitServerAuth("user2", "password2"))
	gs2 := git2.NewTestGitServer(suite.T(), git2.WithTestGitServerFailWhenAuth(true))

	p, ip1, _ := prepareGitIncludeTest(suite.T(), suite.k, mainGs, gs1, gs2)

	var key client.ObjectKey
	if legacyGitSource {
		key = suite.createKluctlDeployment2(p, "test", nil, func(kd *v1beta1.KluctlDeployment) {
			kd.Spec.Source.URL = utils.StrPtr(p.GitUrl())
		})
	} else {
		key = suite.createKluctlDeployment(p, "test", nil)
	}

	suite.Run("fail without authentication", func() {
		suite.waitForCommit(key, "")

		kd := suite.getKluctlDeployment(key)

		readinessCondition := suite.getReadiness(kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionFalse))
		g.Expect(readinessCondition.Reason).To(Equal("PrepareFailed"))
		g.Expect(readinessCondition.Message).To(Equal("failed to clone git source: authentication required"))
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
		if legacyGitSource {
			var credentials []v1beta1.ProjectCredentialsGitDeprecated
			credentials = append(credentials, v1beta1.ProjectCredentialsGitDeprecated{
				Host:       gs1.GitHost(),
				PathPrefix: ip1.GitUrlPath(),
				SecretRef:  v1beta1.LocalObjectReference{Name: secret2},
			})
			credentials = append(credentials, v1beta1.ProjectCredentialsGitDeprecated{
				Host:      mainGs.GitHost(),
				SecretRef: v1beta1.LocalObjectReference{Name: secret1},
			})
			// make sure this one is ignored for http based urls
			credentials = append(credentials, v1beta1.ProjectCredentialsGitDeprecated{
				Host:      "*",
				SecretRef: v1beta1.LocalObjectReference{Name: secret1},
			})
			kd.Spec.Source.Credentials = credentials
		} else {
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
		}
	})

	suite.Run("retry with authentication", func() {
		kd := suite.waitForCommit(key, getHeadRevision(suite.T(), p))

		readinessCondition := suite.getReadiness(kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionTrue))
	})
}

func (suite *GitOpsIncludesSuite) TestGitOpsGitIncludeCredentials() {
	suite.testGitOpsGitIncludeCredentials(false)
}

func (suite *GitOpsIncludesSuite) TestGitOpsGitIncludeCredentialsLegacy() {
	suite.testGitOpsGitIncludeCredentials(true)
}
