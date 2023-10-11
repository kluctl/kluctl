package e2e

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func (suite *GitopsTestSuite) testHelmPull(tc helmTestCase, prePull bool) {
	g := NewWithT(suite.T())

	p, repo, err := prepareHelmTestCase(suite.T(), suite.k, tc, prePull)
	if err != nil {
		if tc.expectedError == "" {
			assert.Fail(suite.T(), "did not expect error")
		}
		return
	}

	createNamespace(suite.T(), suite.k, p.TestSlug()+"-gitops")

	createSecret := func(name string, m map[string]string) {
		mb := map[string][]byte{}
		for k, v := range m {
			mb[k] = []byte(v)
		}
		credsSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: p.TestSlug() + "-gitops",
				Name:      name,
			},
			Data: mb,
		}
		err := suite.k.Client.Create(context.TODO(), credsSecret)
		g.Expect(err).To(Succeed())
	}

	kdModSource := func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.Source = kluctlv1.ProjectSource{
			Git: &kluctlv1.ProjectSourceGit{
				URL: *types2.ParseGitUrlMust(p.GitUrl()),
			},
		}
	}
	var kdModCreds func(kd *kluctlv1.KluctlDeployment)
	if tc.argCredsId != "" {
		createSecret("helm-secrets-1", map[string]string{
			"credentialsId": tc.argCredsId,
			"username":      tc.argUsername,
			"password":      tc.argPassword,
		})
		kdModCreds = func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.HelmCredentials = append(kd.Spec.HelmCredentials, kluctlv1.HelmCredentials{
				SecretRef: kluctlv1.LocalObjectReference{Name: "helm-secrets-1"},
			})
		}
	} else if tc.argCredsHost != "" {
		host := strings.ReplaceAll(tc.argCredsHost, "<host>", repo.URL.Host)
		if tc.oci {
			createSecret("oci-secrets-1", map[string]string{
				"username": tc.argUsername,
				"password": tc.argPassword,
			})
			kdModCreds = func(kd *kluctlv1.KluctlDeployment) {
				kd.Spec.Credentials.Oci = append(kd.Spec.Credentials.Oci, kluctlv1.ProjectCredentialsOci{
					Registry:   host,
					Repository: tc.argCredsPath,
					SecretRef:  kluctlv1.LocalObjectReference{Name: "oci-secrets-1"},
				})
			}
		} else {
			createSecret("helm-secrets-1", map[string]string{
				"username": tc.argUsername,
				"password": tc.argPassword,
			})
			kdModCreds = func(kd *kluctlv1.KluctlDeployment) {
				kd.Spec.Credentials.Helm = append(kd.Spec.Credentials.Helm, kluctlv1.ProjectCredentialsHelm{
					Host:      host,
					Path:      tc.argCredsPath,
					SecretRef: kluctlv1.LocalObjectReference{Name: "helm-secrets-1"},
				})
			}
		}
	}

	key := suite.createKluctlDeployment2(p, "test", map[string]any{
		"namespace": p.TestSlug(),
	}, kdModSource, kdModCreds)

	suite.waitForCommit(key, getHeadRevision(suite.T(), p))

	var kd kluctlv1.KluctlDeployment
	err = suite.k.Client.Get(context.TODO(), key, &kd)
	g.Expect(err).To(Succeed())

	readinessCondition := suite.getReadiness(&kd)
	g.Expect(readinessCondition).ToNot(BeNil())

	if tc.expectedError == "" {
		g.Expect(kd.Status.LastDeployResult).ToNot(BeNil())
		g.Expect(readinessCondition.Status).ToNot(Equal(metav1.ConditionFalse))
		assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "test-helm1-test-chart1")
	} else {
		g.Expect(kd.Status.LastDeployResult).To(BeNil())

		g.Expect(readinessCondition.Status).To(Equal(metav1.ConditionFalse))
		g.Expect(readinessCondition.Reason).To(Equal(kluctlv1.PrepareFailedReason))
		g.Expect(readinessCondition.Message).To(ContainSubstring(tc.expectedError))
	}
}

func (suite *GitopsTestSuite) TestHelm() {
	for _, tc := range helmTests {
		tc := tc
		if tc.name == "dep-oci-creds-fail" {
			continue
		}
		suite.Run(tc.name, func() {
			suite.testHelmPull(tc, false)
		})
	}
}

func (suite *GitopsTestSuite) TestHelmPrePull() {
	for _, tc := range helmTests {
		tc := tc
		if tc.name == "dep-oci-creds-fail" {
			continue
		}
		suite.Run(tc.name, func() {
			suite.testHelmPull(tc, true)
		})
	}
}
