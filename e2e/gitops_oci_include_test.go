package e2e

import (
	"context"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (suite *GitopsTestSuite) TestGitOpsOciIncludeCredentials() {
	g := NewWithT(suite.T())
	_ = g

	ip1 := prepareIncludeProject(suite.T(), "include1", "", nil)
	ip2 := prepareIncludeProject(suite.T(), "include2", "subDir", nil)
	ip3 := prepareIncludeProject(suite.T(), "include3", "", nil)

	ociUrl0 := test_utils.CreateOciRegistry(suite.T(), "user0", "pass0")
	ociUrl1 := test_utils.CreateOciRegistry(suite.T(), "user1", "pass1")
	ociUrl2 := test_utils.CreateOciRegistry(suite.T(), "user2", "pass2")
	ociUrl3 := test_utils.CreateOciRegistry(suite.T(), "user3", "pass3")
	repo0 := ociUrl0.String() + "/org0/repo0"
	repo1 := ociUrl1.String() + "/org1/repo1"
	repo2 := ociUrl2.String() + "/org2/repo2"
	repo3 := ociUrl3.String() + "/org3/repo3"

	ip1.KluctlMust("oci", "push", "--url", repo1, "--creds", "user1:pass1")
	ip2.KluctlMust("oci", "push", "--url", repo2, "--project-dir", ip2.LocalRepoDir(), "--creds", "user2:pass2")
	ip3.KluctlMust("oci", "push", "--url", repo3, "--creds", "user3:pass3")

	p := test_project.NewTestProject(suite.T())

	createNamespace(suite.T(), suite.k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {})

	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"oci": map[string]any{
			"url": repo1,
		},
	}))
	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"oci": map[string]any{
			"url":    repo2,
			"subDir": "subDir",
		},
	}))
	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"oci": map[string]any{
			"url": repo3,
		},
	}))
	p.KluctlMust("oci", "push", "--url", repo0, "--creds", "user0:pass0")

	key := suite.createKluctlDeployment2(p, "test", nil, v1beta1.ProjectSource{
		Oci: &v1beta1.ProjectSourceOci{
			URL: repo0,
		},
	})

	suite.Run("fail without authentication", func() {
		suite.waitForCommit(key, "")

		var kd v1beta1.KluctlDeployment
		err := suite.k.Client.Get(context.Background(), key, &kd)
		g.Expect(err).To(Succeed())

		readinessCondition := suite.getReadiness(&kd)
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
				"username": []byte(username),
				"password": []byte(password),
			},
		}
		err := suite.k.Client.Create(context.Background(), &secret)
		g.Expect(err).To(Succeed())
	}

	createSecret("secret0", "user0", "pass0")
	createSecret("secret1", "user1", "pass1")
	createSecret("secret2", "user2", "pass2")
	createSecret("secret3", "user3", "pass3")

	var kd v1beta1.KluctlDeployment
	err := suite.k.Client.Get(context.Background(), key, &kd)
	g.Expect(err).To(Succeed())

	patch := client.MergeFrom(kd.DeepCopy())

	kd.Spec.Source.Oci.Credentials = append(kd.Spec.Source.Oci.Credentials, v1beta1.ProjectSourceOciCredentials{
		Registry:   ociUrl0.Host,
		Repository: "org0/repo0",
		SecretRef:  v1beta1.LocalObjectReference{Name: "secret0"},
	})
	kd.Spec.Source.Oci.Credentials = append(kd.Spec.Source.Oci.Credentials, v1beta1.ProjectSourceOciCredentials{
		Registry:   ociUrl1.Host,
		Repository: "org1/repo1",
		SecretRef:  v1beta1.LocalObjectReference{Name: "secret1"},
	})
	kd.Spec.Source.Oci.Credentials = append(kd.Spec.Source.Oci.Credentials, v1beta1.ProjectSourceOciCredentials{
		Registry:   ociUrl2.Host,
		Repository: "*/repo2",
		SecretRef:  v1beta1.LocalObjectReference{Name: "secret2"},
	})
	kd.Spec.Source.Oci.Credentials = append(kd.Spec.Source.Oci.Credentials, v1beta1.ProjectSourceOciCredentials{
		Registry:   ociUrl3.Host,
		Repository: "org3/*",
		SecretRef:  v1beta1.LocalObjectReference{Name: "secret3"},
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
