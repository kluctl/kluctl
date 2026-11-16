package e2e

import (
	"testing"

	"github.com/go-git/go-git/v6/plumbing"
	gittypes "github.com/kluctl/kluctl/lib/git/types"
	"github.com/kluctl/kluctl/v2/api/v1beta1"
	git2 "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
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

func (suite *GitOpsIncludesSuite) TestGitSourceRawRef() {
	t := suite.T()
	g := NewWithT(t)
	p := test_project.NewTestProject(t)
	createNamespace(t, suite.k, p.TestSlug())
	p.UpdateTarget("test", nil)

	const ref = "refs/merge-requests/123/merge"
	setValue := func(value string) string {
		return createRawRefCommit(t, p, ref, func() {
			addConfigMapDeployment(p, "cm", map[string]string{"value": value}, resourceOpts{
				name: "raw-ref", namespace: p.TestSlug(),
			})
		})
	}
	firstCommit := setValue("first")
	key := suite.createKluctlDeployment2(p, "test", nil, func(kd *v1beta1.KluctlDeployment) {
		kd.Spec.Source.Git = &v1beta1.ProjectSourceGit{
			URL: p.GitUrl(),
			Ref: &gittypes.GitRef{Raw: ref},
		}
	})
	check := func(commit, value string) {
		kd := suite.waitForCommit(key, commit)
		readiness := suite.getReadiness(kd)
		g.Expect(readiness).ToNot(BeNil())
		g.Expect(readiness.Status).To(Equal(v1.ConditionTrue))
		cm := assertConfigMapExists(t, suite.k, p.TestSlug(), "raw-ref")
		assertNestedFieldEquals(t, cm, value, "data", "value")
	}
	check(firstCommit, "first")
	check(setValue("second"), "second")
	err := p.GetGitRepo().Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(ref), plumbing.NewHash(firstCommit)))
	assert.NoError(t, err)
	check(firstCommit, "first")
}
