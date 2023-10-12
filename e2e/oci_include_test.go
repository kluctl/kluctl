package e2e

import (
	"fmt"
	"github.com/docker/cli/cli/config/configfile"
	"github.com/docker/cli/cli/config/types"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	cp "github.com/otiai10/copy"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOciIncludeMultipleRepos(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	ip1 := prepareIncludeProject(t, "include1", "", nil)
	ip2 := prepareIncludeProject(t, "include2", "subDir", nil)

	repo := &test_utils.TestHelmRepo{
		Oci: true,
	}
	repo.Start(t)

	repo1 := repo.URL.String() + "/org1/repo1"
	repo2 := repo.URL.String() + "/org2/repo2"

	ip1.KluctlMust("oci", "push", "--url", repo1)
	ip2.KluctlMust("oci", "push", "--url", repo2, "--project-dir", ip2.LocalRepoDir())

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

	p.KluctlMust("deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertConfigMapExists(t, k, p.TestSlug(), "include2-cm")
}

func TestOciIncludeWithCreds(t *testing.T) {
	k := defaultCluster1

	ip1 := prepareIncludeProject(t, "include1", "", nil)
	ip2 := prepareIncludeProject(t, "include2", "subDir", nil)
	ip3 := prepareIncludeProject(t, "include3", "", nil)

	createRepo := func(user, pass string) *test_utils.TestHelmRepo {
		repo := &test_utils.TestHelmRepo{
			TestHttpServer: test_utils.TestHttpServer{
				Username: user,
				Password: pass,
			},
			Oci: true,
		}
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
	_, stderr, err := ip1.Kluctl("oci", "push", "--url", repoUrl1)
	assert.ErrorContains(t, err, "401 Unauthorized")
	assert.Contains(t, stderr, "401 Unauthorized")

	// push with invalid creds
	_, stderr, err = ip1.Kluctl("oci", "push", "--url", repoUrl1, "--creds", "user1:invalid")
	assert.ErrorContains(t, err, "401 Unauthorized")
	assert.Contains(t, stderr, "401 Unauthorized")

	// now with valid creds
	ip1.KluctlMust("oci", "push", "--url", repoUrl1, "--creds", "user1:pass1")
	ip2.KluctlMust("oci", "push", "--url", repoUrl2, "--project-dir", ip2.LocalRepoDir(), "--creds", "user2:pass2")
	ip3.KluctlMust("oci", "push", "--url", repoUrl3, "--creds", "user3:pass3")

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
	_, stderr, err = p.Kluctl("deploy", "--yes", "-t", "test")
	assert.ErrorContains(t, err, "401 Unauthorized")
	assert.Contains(t, stderr, "401 Unauthorized")

	// deploy with some invalid creds
	t.Setenv("KLUCTL_REGISTRY_1_HOST", repo1.URL.Host)
	t.Setenv("KLUCTL_REGISTRY_1_REPOSITORY", "org1/repo1")
	t.Setenv("KLUCTL_REGISTRY_1_USERNAME", "user1")
	t.Setenv("KLUCTL_REGISTRY_1_PASSWORD", "pass1")
	t.Setenv("KLUCTL_REGISTRY_2_REPOSITORY", fmt.Sprintf("%s/org2/repo2", repo2.URL.Host))
	t.Setenv("KLUCTL_REGISTRY_2_USERNAME", "user2")
	t.Setenv("KLUCTL_REGISTRY_2_PASSWORD", "invalid")
	_, stderr, err = p.Kluctl("deploy", "--yes", "-t", "test")
	assert.ErrorContains(t, err, "401 Unauthorized")
	assert.Contains(t, stderr, "401 Unauthorized")

	// deploy with valid creds
	t.Setenv("KLUCTL_REGISTRY_2_PASSWORD", "pass2")
	dockerLogin(t, repo3.URL.Host, "user3", "pass3")

	p.KluctlMust("deploy", "--yes", "-t", "test")
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
		Oci: true,
	}
	repo.Start(t)

	repo1 := repo.URL.String() + "/org1/repo1"

	ip1.KluctlMust("oci", "push", "--url", repo1+":tag1")

	ip1.UpdateYaml("cm/configmap-include1-cm.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("v2", "data", "a")
		return nil
	}, "")
	ip1.KluctlMust("oci", "push", "--url", repo1+":tag2")

	ip1.UpdateYaml("cm/configmap-include1-cm.yml", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("v3", "data", "a")
		return nil
	}, "")
	stdout, _ := ip1.KluctlMust("oci", "push", "--url", repo1+":tag3", "--output", "json")
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

	p.KluctlMust("deploy", "--yes", "-t", "test")
	cm := assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "v", "data", "a")

	p.UpdateDeploymentItems("", func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
		_ = items[0].SetNestedField(map[string]any{
			"tag": "tag2",
		}, "oci", "ref")
		return items
	})

	p.KluctlMust("deploy", "--yes", "-t", "test")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "v2", "data", "a")

	p.UpdateDeploymentItems("", func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
		_ = items[0].SetNestedField(map[string]any{
			"digest": digest,
		}, "oci", "ref")
		return items
	})

	p.KluctlMust("deploy", "--yes", "-t", "test")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "v3", "data", "a")
}

func TestLocalOciOverride(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	ip1 := prepareIncludeProject(t, "include1", "", nil)
	ip2 := prepareIncludeProject(t, "include2", "subDir", nil)

	repo := test_utils.TestHelmRepo{
		Oci: true,
	}
	repo.Start(t)

	repo1 := repo.URL.String() + "/org1/repo1"
	repo2 := repo.URL.String() + "/org2/repo2"

	ip1.KluctlMust("oci", "push", "--url", repo1)
	ip2.KluctlMust("oci", "push", "--url", repo2, "--project-dir", ip2.LocalRepoDir())

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

	override1 := t.TempDir()
	err := cp.Copy(ip1.LocalRepoDir(), override1)
	assert.NoError(t, err)

	override2 := t.TempDir()
	err = cp.Copy(ip2.LocalRepoDir(), override2)
	assert.NoError(t, err)

	cm, err := uo.FromFile(filepath.Join(override1, "cm", "configmap-include1-cm.yml"))
	assert.NoError(t, err)
	_ = cm.SetNestedField("o1", "data", "a")
	_ = yaml.WriteYamlFile(filepath.Join(override1, "cm", "configmap-include1-cm.yml"), cm)

	cm, err = uo.FromFile(filepath.Join(override2, "subDir", "cm", "configmap-include2-cm.yml"))
	assert.NoError(t, err)
	_ = cm.SetNestedField("o2", "data", "a")
	_ = yaml.WriteYamlFile(filepath.Join(override2, "subDir", "cm", "configmap-include2-cm.yml"), cm)

	k1 := strings.TrimPrefix(repo1, "oci://")
	k2 := strings.TrimPrefix(repo2, "oci://")

	p.KluctlMust("deploy", "--yes", "-t", "test",
		"--local-oci-override", fmt.Sprintf("%s=%s", k1, override1),
		"--local-oci-override", fmt.Sprintf("%s=%s", k2, override2),
	)
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "o1", "data", "a")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include2-cm")
	assertNestedFieldEquals(t, cm, "o2", "data", "a")
}

func TestLocalOciGroupOverride(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	ip1 := prepareIncludeProject(t, "include1", "", nil)
	ip2 := prepareIncludeProject(t, "include2", "subDir", nil)

	repo := test_utils.TestHelmRepo{
		Oci: true,
	}
	repo.Start(t)

	repo1 := repo.URL.String() + "/org1/repo1"
	repo2 := repo.URL.String() + "/org1/repo2"

	ip1.KluctlMust("oci", "push", "--url", repo1)
	ip2.KluctlMust("oci", "push", "--url", repo2, "--project-dir", ip2.LocalRepoDir())

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

	overrideGroupDir := t.TempDir()

	override1 := filepath.Join(overrideGroupDir, "repo1")
	err := cp.Copy(ip1.LocalRepoDir(), override1)
	assert.NoError(t, err)

	override2 := filepath.Join(overrideGroupDir, "repo2")
	err = cp.Copy(ip2.LocalRepoDir(), override2)
	assert.NoError(t, err)

	cm, err := uo.FromFile(filepath.Join(override1, "cm", "configmap-include1-cm.yml"))
	assert.NoError(t, err)
	_ = cm.SetNestedField("o1", "data", "a")
	_ = yaml.WriteYamlFile(filepath.Join(override1, "cm", "configmap-include1-cm.yml"), cm)

	cm, err = uo.FromFile(filepath.Join(override2, "subDir", "cm", "configmap-include2-cm.yml"))
	assert.NoError(t, err)
	_ = cm.SetNestedField("o2", "data", "a")
	_ = yaml.WriteYamlFile(filepath.Join(override2, "subDir", "cm", "configmap-include2-cm.yml"), cm)

	k1 := strings.TrimPrefix(repo.URL.String(), "oci://") + "/org1"

	p.KluctlMust("deploy", "--yes", "-t", "test",
		"--local-oci-group-override", fmt.Sprintf("%s=%s", k1, overrideGroupDir),
	)
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include1-cm")
	assertNestedFieldEquals(t, cm, "o1", "data", "a")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "include2-cm")
	assertNestedFieldEquals(t, cm, "o2", "data", "a")
}
