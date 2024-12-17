package e2e

import (
	"context"
	"fmt"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/process"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars/sops_test_resources"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/suite"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"path"
	"strings"
	"testing"
)

type GitOpsSopsSuite struct {
	GitopsTestSuite
}

func TestGitOpsSops(t *testing.T) {
	suite.Run(t, new(GitOpsSopsSuite))
}

func (suite *GitOpsSopsSuite) TestEncryptedVars() {
	g := NewWithT(suite.T())

	p := test_project.NewTestProject(suite.T())
	createNamespace(suite.T(), suite.k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm", map[string]string{
		"v1": "{{ test1.test2 }}",
	}, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})
	p.UpdateDeploymentYaml("", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField([]map[string]any{
			{
				"file": "encrypted-vars.yaml",
			},
		}, "vars")
		return nil
	})

	p.UpdateFile("encrypted-vars.yaml", func(f string) (string, error) {
		b, _ := sops_test_resources.TestResources.ReadFile("test.yaml")
		return string(b), nil
	}, "")

	key := suite.createKluctlDeployment(p, "test", nil)

	suite.Run("deployment with missing sops key fails", func() {
		kd := suite.waitForCommit(key, getHeadRevision(suite.T(), p))
		readinessCondition := suite.getReadiness(kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionFalse))

		g.Expect(kd.Status.ReconcileRequestResult.CommandError).To(ContainSubstring("cannot get sops data key"))
		assertConfigMapNotExists(suite.T(), suite.k, p.TestSlug(), "cm")
	})

	sopsKey, _ := sops_test_resources.TestResources.ReadFile("test-key.txt")
	secret := corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "sops-secrets-age",
			Namespace: key.Namespace,
		},
		Data: map[string][]byte{
			"identity.agekey": sopsKey,
		},
	}
	err := suite.k.Client.Create(context.Background(), &secret)
	g.Expect(err).To(Succeed())

	suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.Decryption = &kluctlv1.Decryption{
			Provider: "sops",
			SecretRef: &kluctlv1.LocalObjectReference{
				Name: secret.Name,
			},
		}
	})

	suite.Run("deployment with existing sops key", func() {
		kd := suite.waitForReconcile(key)
		readinessCondition := suite.getReadiness(kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionTrue))

		cm := assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm")
		assertNestedFieldEquals(suite.T(), cm, map[string]any{
			"v1": "42",
		}, "data")
	})
}

func (suite *GitOpsSopsSuite) TestGpgAgentKilled() {
	g := NewWithT(suite.T())

	p := test_project.NewTestProject(suite.T())
	createNamespace(suite.T(), suite.k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm", map[string]string{
		"v1": "{{ test1.test2 }}",
	}, resourceOpts{
		name:      "cm",
		namespace: p.TestSlug(),
	})
	p.UpdateDeploymentYaml("", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField([]map[string]any{
			{
				"file": "encrypted-vars.yaml",
			},
		}, "vars")
		return nil
	})

	p.UpdateFile("encrypted-vars.yaml", func(f string) (string, error) {
		b, _ := sops_test_resources.TestResources.ReadFile("test-gpg.yaml")
		return string(b), nil
	}, "")

	countGpgAgents := func() int {
		pss, err := process.ListProcesses(context.Background())
		g.Expect(err).To(Succeed())
		count := 0
		suite.T().Logf("counting gpg-agents")
		for _, ps := range pss {
			homeDirPrefix := path.Join(os.TempDir(), "gpg-")
			homeDirArg := fmt.Sprintf("--homedir %s", homeDirPrefix)
			if strings.Index(ps.Command, "gpg-agent") != -1 && strings.Index(ps.Command, homeDirArg) != -1 {
				suite.T().Logf("gpg-agent: pid=%d, cmd=%s", ps.Pid, ps.Command)
				count++
			}
		}
		return count
	}

	gpgAgentCount := countGpgAgents()

	key := suite.createKluctlDeployment(p, "test", nil)

	gpgKey, _ := sops_test_resources.TestResources.ReadFile("private.gpg")
	secret := corev1.Secret{
		ObjectMeta: v1.ObjectMeta{
			Name:      "sops-secrets-gpg",
			Namespace: key.Namespace,
		},
		Data: map[string][]byte{
			"identity.asc": gpgKey,
		},
	}
	err := suite.k.Client.Create(context.Background(), &secret)
	g.Expect(err).To(Succeed())

	suite.updateKluctlDeployment(key, func(kd *kluctlv1.KluctlDeployment) {
		kd.Spec.Decryption = &kluctlv1.Decryption{
			Provider: "sops",
			SecretRef: &kluctlv1.LocalObjectReference{
				Name: secret.Name,
			},
		}
	})

	suite.Run("deployment with existing sops key", func() {
		kd := suite.waitForReconcile(key)
		readinessCondition := suite.getReadiness(kd)
		g.Expect(readinessCondition).ToNot(BeNil())
		g.Expect(readinessCondition.Status).To(Equal(v1.ConditionTrue))

		cm := assertConfigMapExists(suite.T(), suite.k, p.TestSlug(), "cm")
		assertNestedFieldEquals(suite.T(), cm, map[string]any{
			"v1": "43",
		}, "data")
	})

	suite.Run("verify gpg-agents got killed", func() {
		newGpgAgentCount := countGpgAgents()
		g.Expect(gpgAgentCount).To(Equal(newGpgAgentCount))
	})
}
