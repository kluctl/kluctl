package e2e

import (
	"context"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	git2 "github.com/kluctl/kluctl/v2/pkg/git"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (suite *GitopsTestSuite) TestGitOpsGitIncludeDeprecatedSecret() {
	g := NewWithT(suite.T())
	_ = g

	mainGs := git2.NewTestGitServer(suite.T(), git2.WithTestGitServerAuth("user1", "password1"))
	gs1 := git2.NewTestGitServer(suite.T(), git2.WithTestGitServerAuth("user1", "password1"))
	gs2 := git2.NewTestGitServer(suite.T())

	p, _, _ := prepareGitIncludeTest(suite.T(), suite.k, mainGs, gs1, gs2)

	key := suite.createKluctlDeployment(p, "test", nil)

	suite.Run("fail without authentication", func() {
		suite.waitForCommit(key, "")

		var kd v1beta1.KluctlDeployment
		err := suite.k.Client.Get(context.Background(), key, &kd)
		g.Expect(err).To(Succeed())

		readinessCondition := suite.getReadiness(&kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionFalse))
		g.Expect(readinessCondition.Reason).To(Equal("PrepareFailed"))
		g.Expect(readinessCondition.Message).To(Equal("failed clone source: authentication required"))
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

	var kd v1beta1.KluctlDeployment
	err = suite.k.Client.Get(context.Background(), key, &kd)
	g.Expect(err).To(Succeed())

	patch := client.MergeFrom(kd.DeepCopy())
	kd.Spec.Source.SecretRef = &v1beta1.LocalObjectReference{Name: "git-secrets"}

	err = suite.k.Client.Patch(context.Background(), &kd, patch, client.FieldOwner("kluctl"))
	g.Expect(err).To(Succeed())

	suite.Run("retry with authentication", func() {
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))

		var kd v1beta1.KluctlDeployment
		err := suite.k.Client.Get(context.Background(), key, &kd)
		g.Expect(err).To(Succeed())

		readinessCondition := suite.getReadiness(&kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionTrue))
	})
}

func (suite *GitopsTestSuite) TestGitOpsGitIncludeCredentials() {
	g := NewWithT(suite.T())
	_ = g

	mainGs := git2.NewTestGitServer(suite.T(), git2.WithTestGitServerAuth("user1", "password1"))
	gs1 := git2.NewTestGitServer(suite.T(), git2.WithTestGitServerAuth("user2", "password2"))
	gs2 := git2.NewTestGitServer(suite.T(), git2.WithTestGitServerFailWhenAuth(true))

	p, ip1, _ := prepareGitIncludeTest(suite.T(), suite.k, mainGs, gs1, gs2)

	key := suite.createKluctlDeployment(p, "test", nil)

	suite.Run("fail without authentication", func() {
		suite.waitForCommit(key, "")

		var kd v1beta1.KluctlDeployment
		err := suite.k.Client.Get(context.Background(), key, &kd)
		g.Expect(err).To(Succeed())

		readinessCondition := suite.getReadiness(&kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionFalse))
		g.Expect(readinessCondition.Reason).To(Equal("PrepareFailed"))
		g.Expect(readinessCondition.Message).To(Equal("failed clone source: authentication required"))
	})

	createSecret := func(secretName string, username string, password string) {
		secret := corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      secretName,
				Namespace: key.Namespace,
			},
			Data: map[string][]byte{
				"username": []byte(username),
				"password": []byte(password),
			},
		}
		err := suite.k.Client.Create(context.Background(), &secret)
		g.Expect(err).To(Succeed())
	}

	createSecret("secret1", "user1", "password1")
	createSecret("secret2", "user2", "password2")

	var kd v1beta1.KluctlDeployment
	err := suite.k.Client.Get(context.Background(), key, &kd)
	g.Expect(err).To(Succeed())

	patch := client.MergeFrom(kd.DeepCopy())

	kd.Spec.Source.Credentials = append(kd.Spec.Source.Credentials, v1beta1.ProjectSourceCredentials{
		Host:       gs1.GitHost(),
		PathPrefix: ip1.GitRepoName(),
		SecretRef:  v1beta1.LocalObjectReference{Name: "secret2"},
	})
	kd.Spec.Source.Credentials = append(kd.Spec.Source.Credentials, v1beta1.ProjectSourceCredentials{
		Host:      mainGs.GitHost(),
		SecretRef: v1beta1.LocalObjectReference{Name: "secret1"},
	})
	// make sure this one is ignored for http based urls
	kd.Spec.Source.Credentials = append(kd.Spec.Source.Credentials, v1beta1.ProjectSourceCredentials{
		Host:      "*",
		SecretRef: v1beta1.LocalObjectReference{Name: "secret1"},
	})

	err = suite.k.Client.Patch(context.Background(), &kd, patch, client.FieldOwner("kluctl"))
	g.Expect(err).To(Succeed())

	suite.Run("retry with authentication", func() {
		suite.waitForCommit(key, getHeadRevision(suite.T(), p))

		var kd v1beta1.KluctlDeployment
		err := suite.k.Client.Get(context.Background(), key, &kd)
		g.Expect(err).To(Succeed())

		readinessCondition := suite.getReadiness(&kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionTrue))
	})
}
