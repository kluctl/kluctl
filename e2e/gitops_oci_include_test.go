package e2e

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

type GitOpsOciIncludeSuite struct {
	GitopsTestSuite
}

func TestGitOpsOciInclude(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GitOpsOciIncludeSuite))
}

func (suite *GitOpsOciIncludeSuite) TestGitOpsOciIncludeCredentials() {
	g := NewWithT(suite.T())
	_ = g

	ip1 := prepareIncludeProject(suite.T(), "include1", "", nil)
	ip2 := prepareIncludeProject(suite.T(), "include2", "subDir", nil)
	ip3 := prepareIncludeProject(suite.T(), "include3", "", nil)

	createRepo := func(user, pass string) *test_utils.TestHelmRepo {
		repo := &test_utils.TestHelmRepo{
			TestHttpServer: test_utils.TestHttpServer{
				Username: user,
				Password: pass,
			},
			Oci: true,
		}
		repo.Start(suite.T())
		return repo
	}

	repo0 := createRepo("user0", "pass0")
	repo1 := createRepo("user1", "pass1")
	repo2 := createRepo("user2", "pass2")
	repo3 := createRepo("user3", "pass3")

	repoUrl0 := repo0.URL.String() + "/org0/repo0"
	repoUrl1 := repo1.URL.String() + "/org1/repo1"
	repoUrl2 := repo2.URL.String() + "/org2/repo2"
	repoUrl3 := repo3.URL.String() + "/org3/repo3"

	ip1.KluctlMust("oci", "push", "--url", repoUrl1, "--registry-creds", fmt.Sprintf("%s=user1:pass1", repo1.URL.Host))
	ip2.KluctlMust("oci", "push", "--url", repoUrl2, "--project-dir", ip2.LocalRepoDir(), "--registry-creds", fmt.Sprintf("%s=user2:pass2", repo2.URL.Host))
	ip3.KluctlMust("oci", "push", "--url", repoUrl3, "--registry-creds", fmt.Sprintf("%s=user3:pass3", repo3.URL.Host))

	p := test_project.NewTestProject(suite.T())

	createNamespace(suite.T(), suite.k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {})

	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"oci": map[string]any{
			"url": repoUrl1,
		},
	}))
	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"oci": map[string]any{
			"url":    repoUrl2,
			"subDir": "subDir",
		},
	}))
	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"oci": map[string]any{
			"url": repoUrl3,
		},
	}))
	p.KluctlMust("oci", "push", "--url", repoUrl0, "--registry-creds", fmt.Sprintf("%s=user0:pass0", repo0.URL.Host))

	var key = suite.createKluctlDeployment2(p, "test", nil, func(kd *v1beta1.KluctlDeployment) {
		kd.Spec.Source.Oci = &v1beta1.ProjectSourceOci{
			URL: repoUrl0,
		}
	})
	suite.Run("fail without authentication", func() {
		kd := suite.waitForCommit(key, "")

		readinessCondition := suite.getReadiness(kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionFalse))
		g.Expect(readinessCondition.Reason).To(Equal("PrepareFailed"))
		g.Expect(readinessCondition.Message).To(ContainSubstring("401 Unauthorized"))
	})

	createSecret := func(secretName string, username string, password string) {
		secret := corev1.Secret{
			ObjectMeta: v1.ObjectMeta{
				Name:      secretName,
				Namespace: key.Namespace,
			},
			Data: map[string][]byte{
				"username":   []byte(username),
				"password":   []byte(password),
				"plain_http": []byte("true"),
			},
		}
		err := suite.k.Client.Create(context.Background(), &secret)
		g.Expect(err).To(Succeed())
	}

	createSecret("secret0", "user0", "pass0")
	createSecret("secret1", "user1", "pass1")
	createSecret("secret2", "user2", "pass2")
	createSecret("secret3", "user3", "pass3")

	suite.updateKluctlDeployment(key, func(kd *v1beta1.KluctlDeployment) {
		kd.Spec.Credentials.Oci = append(kd.Spec.Credentials.Oci, v1beta1.ProjectCredentialsOci{
			Registry:   repo0.URL.Host,
			Repository: "org0/repo0",
			SecretRef:  v1beta1.LocalObjectReference{Name: "secret0"},
		})
		kd.Spec.Credentials.Oci = append(kd.Spec.Credentials.Oci, v1beta1.ProjectCredentialsOci{
			Registry:   repo1.URL.Host,
			Repository: "org1/repo1",
			SecretRef:  v1beta1.LocalObjectReference{Name: "secret1"},
		})
		kd.Spec.Credentials.Oci = append(kd.Spec.Credentials.Oci, v1beta1.ProjectCredentialsOci{
			Registry:   repo2.URL.Host,
			Repository: "*/repo2",
			SecretRef:  v1beta1.LocalObjectReference{Name: "secret2"},
		})
		kd.Spec.Credentials.Oci = append(kd.Spec.Credentials.Oci, v1beta1.ProjectCredentialsOci{
			Registry:   repo3.URL.Host,
			Repository: "org3/*",
			SecretRef:  v1beta1.LocalObjectReference{Name: "secret3"},
		})
	})

	suite.Run("retry with authentication", func() {
		kd := suite.waitForCommit(key, getHeadRevision(suite.T(), p))

		readinessCondition := suite.getReadiness(kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionTrue))
	})
}
