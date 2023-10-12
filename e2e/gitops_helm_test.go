package e2e

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	types2 "github.com/kluctl/kluctl/v2/pkg/types"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
	"testing"
)

type GitOpsHelmSuite struct {
	GitopsTestSuite
}

func TestGitOpsHelm(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GitOpsHelmSuite))
}

func (suite *GitOpsHelmSuite) testHelmPull(tc helmTestCase, prePull bool) {
	g := NewWithT(suite.T())

	p, repo, err := prepareHelmTestCase(suite.T(), suite.k, tc, prePull)
	if err != nil {
		if tc.expectedError == "" {
			assert.Fail(suite.T(), "did not expect error")
		}
		return
	}

	var legacyHelmCreds []kluctlv1.HelmCredentials
	var projectCreds kluctlv1.ProjectCredentials

	if tc.argCredsId != "" {
		name := suite.createGitopsSecret(map[string]string{
			"credentialsId": tc.argCredsId,
			"username":      tc.argUsername,
			"password":      tc.argPassword,
		})
		legacyHelmCreds = append(legacyHelmCreds, kluctlv1.HelmCredentials{
			SecretRef: kluctlv1.LocalObjectReference{Name: name},
		})
	} else if tc.argCredsHost != "" {
		host := strings.ReplaceAll(tc.argCredsHost, "<host>", repo.URL.Host)
		if tc.oci {
			m := map[string]string{
				"username": tc.argUsername,
				"password": tc.argPassword,
			}
			if !repo.TLSEnabled {
				m["plain_http"] = "true"
			}
			if tc.argPassCA {
				m["ca"] = string(repo.ServerCAs)
			}
			if tc.argPassClientCert {
				m["cert"] = string(repo.ClientCert)
			}
			if tc.argPassClientCert {
				m["key"] = string(repo.ClientKey)
			}
			name := suite.createGitopsSecret(m)
			projectCreds.Oci = append(projectCreds.Oci, kluctlv1.ProjectCredentialsOci{
				Registry:   host,
				Repository: tc.argCredsPath,
				SecretRef:  kluctlv1.LocalObjectReference{Name: name},
			})
		} else {
			m := map[string]string{
				"username": tc.argUsername,
				"password": tc.argPassword,
			}
			if tc.argPassCA {
				m["ca"] = string(repo.ServerCAs)
			}
			if tc.argPassClientCert {
				m["cert"] = string(repo.ClientCert)
			}
			if tc.argPassClientCert {
				m["key"] = string(repo.ClientKey)
			}
			name := suite.createGitopsSecret(m)
			projectCreds.Helm = append(projectCreds.Helm, kluctlv1.ProjectCredentialsHelm{
				Host:      host,
				Path:      tc.argCredsPath,
				SecretRef: kluctlv1.LocalObjectReference{Name: name},
			})
		}
	}

	// add a fallback secret that enables plain_http in case we have no matching creds
	if tc.oci && !repo.TLSEnabled {
		m := map[string]string{
			"plain_http": "true",
		}
		name := suite.createGitopsSecret(m)
		projectCreds.Oci = append(projectCreds.Oci, kluctlv1.ProjectCredentialsOci{
			Registry:  repo.URL.Host,
			SecretRef: kluctlv1.LocalObjectReference{Name: name},
		})
	}

	key := suite.createKluctlDeployment2(p, "test", map[string]any{
		"namespace": p.TestSlug(),
	}, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.Source = kluctlv1.ProjectSource{
			Git: &kluctlv1.ProjectSourceGit{
				URL: *types2.ParseGitUrlMust(p.GitUrl()),
			},
		}
		kd.Spec.HelmCredentials = legacyHelmCreds
		kd.Spec.Credentials = projectCreds
	})

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

func (suite *GitOpsHelmSuite) TestHelm() {
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

func (suite *GitOpsHelmSuite) TestHelmPrePull() {
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
