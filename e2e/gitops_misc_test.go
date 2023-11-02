package e2e

import (
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

type GitOpsMiscSuite struct {
	GitopsTestSuite
}

func TestGitOpsMisc(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(GitOpsMiscSuite))
}

func (suite *GitOpsMiscSuite) TestGitSourceWithPath() {
	p := test_project.NewTestProject(suite.T(), test_project.WithGitSubDir("subDir"))
	p.AddExtraArgs("--controller-namespace", suite.gitopsNamespace+"-system")
	createNamespace(suite.T(), suite.k, p.TestSlug())

	p.UpdateTarget("target1", nil)
	addConfigMapDeployment(p, "d1", nil, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})

	key := suite.createKluctlDeployment(p, "target1", nil)

	suite.Run("initial deployment fails", func() {
		kd := suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		status := suite.getReadiness(kd)
		assert.Equal(suite.T(), v1.ConditionFalse, status.Status)
		assert.Equal(suite.T(), "target target1 not existent in kluctl project config", status.Message)
	})

	suite.Run("deployment with path succeeds", func() {
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Source.Git.Path = "subDir"
		})

		kd := suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		status := suite.getReadiness(kd)
		assert.Equal(suite.T(), v1.ConditionTrue, status.Status)
	})
}

func (suite *GitOpsMiscSuite) TestOciSourceWithPath() {
	p := test_project.NewTestProject(suite.T(), test_project.WithGitSubDir("subDir"))
	createNamespace(suite.T(), suite.k, p.TestSlug())

	p.UpdateTarget("target1", nil)
	addConfigMapDeployment(p, "d1", nil, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})

	repo := &test_utils.TestHelmRepo{
		TestHttpServer: test_utils.TestHttpServer{},
		Oci:            true,
	}
	repo.Start(suite.T())

	repoUrl := repo.URL.String() + "/org/repo"

	p.KluctlMust(suite.T(), "oci", "push", "--url", repoUrl, "--project-dir", p.LocalRepoDir())

	p.AddExtraArgs("--controller-namespace", suite.gitopsNamespace+"-system")

	key := suite.createKluctlDeployment2(p, "target1", nil, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.Source.Oci = &kluctlv1.ProjectSourceOci{
			URL: repoUrl,
		}
	})

	suite.Run("initial deployment fails", func() {
		kd := suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		status := suite.getReadiness(kd)
		assert.Equal(suite.T(), v1.ConditionFalse, status.Status)
		assert.Equal(suite.T(), "target target1 not existent in kluctl project config", status.Message)
	})

	suite.Run("deployment with path succeeds", func() {
		suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
			kd.Spec.Source.Oci.Path = "subDir"
		})

		kd := suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		status := suite.getReadiness(kd)
		assert.Equal(suite.T(), v1.ConditionTrue, status.Status)
	})
}
