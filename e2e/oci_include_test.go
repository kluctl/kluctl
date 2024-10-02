package e2e

import (
	"fmt"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	"github.com/kluctl/kluctl/lib/yaml"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

func TestOciIncludeMultipleRepos(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	ip1 := prepareIncludeProject(t, "include1", "", nil)
	ip2 := prepareIncludeProject(t, "include2", "subDir", nil)

	repo := &test_utils.TestHelmRepo{
		Type: test_utils.TestHelmRepo_Oci,
	}
	repo.Start(t)

	repo1 := repo.URL.String() + "/org1/repo1"
	repo2 := repo.URL.String() + "/org2/repo2"

	ip1.KluctlMust(t, "oci", "push", "--url", repo1)
	ip2.KluctlMust(t, "oci", "push", "--url", repo2, "--project-dir", ip2.LocalWorkDir())

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

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

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertConfigMapExists(t, k, p.TestSlug(), "include2-cm")
}

func TestOciIncludeWithCreds(t *testing.T) {
	k := defaultCluster1

	ip1 := prepareIncludeProject(t, "include1", "", nil)
	ip2 := prepareIncludeProject(t, "include2", "subDir", nil)
	ip3 := prepareIncludeProject(t, "include3", "", nil)

	createRepo := func(user, pass string) *test_utils.TestHelmRepo {
		repo := test_utils.NewHelmTestRepoHttp(test_utils.TestHelmRepo_Oci, "", user, pass, false, false)
		repo.Start(t)
		return repo
	}
	repo1 := createRepo("user1", "pass1")
	repo2 := createRepo("user2", "pass2")
	repo3 := createRepo("user3", "pass3")
	repoUrl1 := repo1.URL.String() + "/org1/repo1"
	repoUrl2 := repo2.URL.String() + "/org2/repo2"
	repoUrl3 := repo3.URL.String() + "/org3/repo3"

	// push with no creds
	_, _, err := ip1.Kluctl(t, "oci", "push", "--url", repoUrl1)
	assert.ErrorContains(t, err, "401 Unauthorized")

	// push with invalid creds
	_, _, err = ip1.Kluctl(t, "oci", "push", "--url", repoUrl1, "--registry-creds", fmt.Sprintf("%s=user1:invalid", repo1.URL.Host))
	assert.ErrorContains(t, err, "401 Unauthorized")

	// now with valid creds
	ip1.KluctlMust(t, "oci", "push", "--url", repoUrl1, "--registry-creds", fmt.Sprintf("%s=user1:pass1", repo1.URL.Host))
	ip2.KluctlMust(t, "oci", "push", "--url", repoUrl2, "--project-dir", ip2.LocalWorkDir(), "--registry-creds", fmt.Sprintf("%s=user2:pass2", repo2.URL.Host))
	ip3.KluctlMust(t, "oci", "push", "--url", repoUrl3, "--registry-creds", fmt.Sprintf("%s=user3:pass3", repo3.URL.Host))

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

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

	// deploy with no auth
	_, _, err = p.Kluctl(t, "deploy", "--yes", "-t", "test")
	assert.ErrorContains(t, err, "401 Unauthorized")

	// deploy with some invalid creds
	t.Setenv("KLUCTL_REGISTRY_1_HOST", repo1.URL.Host)
	t.Setenv("KLUCTL_REGISTRY_1_REPOSITORY", "org1/repo1")
	t.Setenv("KLUCTL_REGISTRY_1_USERNAME", "user1")
	t.Setenv("KLUCTL_REGISTRY_1_PASSWORD", "pass1")
	t.Setenv("KLUCTL_REGISTRY_2_REPOSITORY", fmt.Sprintf("%s/org2/repo2", repo2.URL.Host))
	t.Setenv("KLUCTL_REGISTRY_2_USERNAME", "user2")
	t.Setenv("KLUCTL_REGISTRY_2_PASSWORD", "invalid")
	_, _, err = p.Kluctl(t, "deploy", "--yes", "-t", "test")
	assert.ErrorContains(t, err, "401 Unauthorized")

	// deploy with valid creds
	t.Setenv("KLUCTL_REGISTRY_2_PASSWORD", "pass2")
	dockerLogin(t, repo3.URL.Host, "user3", "pass3")

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertConfigMapExists(t, k, p.TestSlug(), "include2-cm")
}

func dockerLogin(t *testing.T, host string, username string, password string) {
	dir := t.TempDir()

	var cf configfile.ConfigFile
	cf.AuthConfigs = map[string]types.AuthConfig{
		host: {
			Username: username,
			Password: password,
		},
	}

	s := yaml.WriteJsonStringMust(&cf)

	err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(s), 0600)
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("DOCKER_CONFIG", dir)
}

func TestOciIncludeTagsAndDigests(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	ip1 := prepareIncludeProject(t, "include1", "", nil)

	repo := test_utils.TestHelmRepo{
		Type: test_utils.TestHelmRepo_Oci,
	}
	repo.Start(t)

	repo1 := repo.URL.String() + "/org1/repo1"

	ip1.KluctlMust(t, "oci", "push", "--url", repo1+":tag1")

	ip1.UpdateYaml("cm/configmap-include1-cm.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("v2", "data", "a")
		return nil
	}, "")
	ip1.KluctlMust(t, "oci", "push", "--url", repo1+":tag2")

	ip1.UpdateYaml("cm/configmap-include1-cm.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("v3", "data", "a")
		return nil
	}, "")
	stdout, _ := ip1.KluctlMust(t, "oci", "push", "--url", repo1+":tag3", "--output", "json")
	md := uo.FromStringMust(stdout)
	digest, _, _ := md.GetNestedString("digest")

	p := test_project.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {})

	p.AddDeploymentItem("", uo.FromMap(map[string]interface{}{
		"oci": map[string]any{
			"url": repo1,
			"ref": map[string]any{
				"tag": "tag1",
			},
		},
	}))

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm := assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "v", "data", "a")

	p.UpdateDeploymentItems("", func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
		_ = items[0].SetNestedField(map[string]any{
			"tag": "tag2",
		}, "oci", "ref")
		return items
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "v2", "data", "a")

	p.UpdateDeploymentItems("", func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
		_ = items[0].SetNestedField(map[string]any{
			"digest": digest,
		}, "oci", "ref")
		return items
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "v3", "data", "a")
}
