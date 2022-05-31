package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/internal/test-utils"
	"path/filepath"
	"reflect"
	"testing"
)

func prepareInclusionTestProject(t *testing.T, namespace string, withIncludes bool) (*testProject, *test_utils.KindCluster) {
	isDone := false

	k := defaultKindCluster1
	p := &testProject{}
	p.init(t, k, namespace)
	defer func() {
		if !isDone {
			p.cleanup()
		}
	}()

	recreateNamespace(t, k, p.projectName)

	p.updateTarget("test", nil)

	addConfigMapDeployment(p, "cm1", nil, resourceOpts{name: "cm1", namespace: p.projectName})
	addConfigMapDeployment(p, "cm2", nil, resourceOpts{name: "cm2", namespace: p.projectName})
	addConfigMapDeployment(p, "cm3", nil, resourceOpts{name: "cm3", namespace: p.projectName, tags: []string{"tag1", "tag2"}})
	addConfigMapDeployment(p, "cm4", nil, resourceOpts{name: "cm4", namespace: p.projectName, tags: []string{"tag1", "tag3"}})
	addConfigMapDeployment(p, "cm5", nil, resourceOpts{name: "cm5", namespace: p.projectName, tags: []string{"tag1", "tag4"}})
	addConfigMapDeployment(p, "cm6", nil, resourceOpts{name: "cm6", namespace: p.projectName, tags: []string{"tag1", "tag5"}})
	addConfigMapDeployment(p, "cm7", nil, resourceOpts{name: "cm7", namespace: p.projectName, tags: []string{"tag1", "tag6"}})

	if withIncludes {
		p.addDeploymentInclude(".", "include1", nil)
		addConfigMapDeployment(p, "include1/icm1", nil, resourceOpts{name: "icm1", namespace: p.projectName, tags: []string{"itag1", "itag2"}})

		p.addDeploymentInclude(".", "include2", nil)
		addConfigMapDeployment(p, "include2/icm2", nil, resourceOpts{name: "icm2", namespace: p.projectName})
		addConfigMapDeployment(p, "include2/icm3", nil, resourceOpts{name: "icm3", namespace: p.projectName, tags: []string{"itag3", "itag4"}})

		p.addDeploymentInclude(".", "include3", []string{"itag5"})
		addConfigMapDeployment(p, "include3/icm4", nil, resourceOpts{name: "icm4", namespace: p.projectName})
		addConfigMapDeployment(p, "include3/icm5", nil, resourceOpts{name: "icm5", namespace: p.projectName, tags: []string{"itag5", "itag6"}})
	}

	isDone = true
	return p, k
}

func assertExistsHelper(t *testing.T, p *testProject, k *test_utils.KindCluster, shouldExists map[string]bool, add []string, remove []string) {
	for _, x := range add {
		shouldExists[x] = true
	}
	for _, x := range remove {
		if _, ok := shouldExists[x]; ok {
			delete(shouldExists, x)
		}
	}
	exists := k.KubectlYamlMust(t, "-n", p.projectName, "get", "configmaps", "-l", fmt.Sprintf("project_name=%s", p.projectName))
	items, _, _ := exists.GetNestedObjectList("items")
	found := make(map[string]bool)
	for _, x := range items {
		found[x.GetK8sName()] = true
	}
	if !reflect.DeepEqual(shouldExists, found) {
		t.Errorf("found != shouldExists")
	}
}

func TestInclusionTags(t *testing.T) {
	t.Parallel()
	p, k := prepareInclusionTestProject(t, "inclusion-tags", false)
	defer p.cleanup()

	shouldExists := make(map[string]bool)
	doAssertExists := func(add ...string) {
		assertExistsHelper(t, p, k, shouldExists, add, nil)
	}

	doAssertExists()

	// test default tags
	p.KluctlMust("deploy", "--yes", "-t", "test", "-I", "cm1")
	doAssertExists("cm1")
	p.KluctlMust("deploy", "--yes", "-t", "test", "-I", "cm2")
	doAssertExists("cm2")

	// cm3/cm4 don't have default tags, so they should not deploy
	p.KluctlMust("deploy", "--yes", "-t", "test", "-I", "cm3")
	doAssertExists()

	// but with tag2, at least cm3 should deploy
	p.KluctlMust("deploy", "--yes", "-t", "test", "-I", "tag2")
	doAssertExists("cm3")

	// let's try 2 tags at once
	p.KluctlMust("deploy", "--yes", "-t", "test", "-I", "tag3", "-I", "tag4")
	doAssertExists("cm4", "cm5")

	// And now let's try a tag that matches all non-default ones
	p.KluctlMust("deploy", "--yes", "-t", "test", "-I", "tag3", "-I", "tag1")
	doAssertExists("cm6", "cm7")
}

func TestExclusionTags(t *testing.T) {
	t.Parallel()
	p, k := prepareInclusionTestProject(t, "inclusion-exclusion", false)
	defer p.cleanup()

	shouldExists := make(map[string]bool)
	doAssertExists := func(add ...string) {
		assertExistsHelper(t, p, k, shouldExists, add, nil)
	}

	doAssertExists()

	// Exclude everything except cm1
	p.KluctlMust("deploy", "--yes", "-t", "test", "-E", "cm2", "-E", "tag1")
	doAssertExists("cm1")

	// Test that exclusion has precedence over inclusion
	p.KluctlMust("deploy", "--yes", "-t", "test", "-E", "cm2", "-E", "tag1", "-I", "cm2")
	doAssertExists()

	// Test that exclusion has precedence over inclusion
	p.KluctlMust("deploy", "--yes", "-t", "test", "-I", "tag1", "-E", "tag6")
	doAssertExists("cm3", "cm4", "cm5", "cm6")
}

func TestInclusionIncludeDirs(t *testing.T) {
	t.Parallel()
	p, k := prepareInclusionTestProject(t, "inclusion-dirs", true)
	defer p.cleanup()

	shouldExists := make(map[string]bool)
	doAssertExists := func(add ...string) {
		assertExistsHelper(t, p, k, shouldExists, add, nil)
	}

	doAssertExists()

	p.KluctlMust("deploy", "--yes", "-t", "test", "-I", "itag1")
	doAssertExists("icm1")

	p.KluctlMust("deploy", "--yes", "-t", "test", "-I", "include2")
	doAssertExists("icm2", "icm3")

	p.KluctlMust("deploy", "--yes", "-t", "test", "-I", "itag5")
	doAssertExists("icm4", "icm5")
}

func TestInclusionDeploymentDirs(t *testing.T) {
	t.Parallel()
	p, k := prepareInclusionTestProject(t, "inclusion-kustomize-dirs", true)
	defer p.cleanup()

	shouldExists := make(map[string]bool)
	doAssertExists := func(add ...string) {
		assertExistsHelper(t, p, k, shouldExists, add, nil)
	}

	doAssertExists()

	p.KluctlMust("deploy", "--yes", "-t", "test", "--include-deployment-dir", "include1/icm1")
	doAssertExists("icm1")

	p.KluctlMust("deploy", "--yes", "-t", "test", "--include-deployment-dir", "include2/icm3")
	doAssertExists("icm3")

	p.KluctlMust("deploy", "--yes", "-t", "test", "--exclude-deployment-dir", "include3/icm5")
	var a []string
	for _, x := range p.listDeploymentItemPathes(".", false) {
		if x != "icm5" {
			a = append(a, filepath.Base(x))
		}
	}
	doAssertExists(a...)
}

func TestInclusionPrune(t *testing.T) {
	t.Parallel()
	p, k := prepareInclusionTestProject(t, "inclusion-prune", false)
	defer p.cleanup()

	shouldExists := make(map[string]bool)
	doAssertExists := func(add []string, remove []string) {
		assertExistsHelper(t, p, k, shouldExists, add, remove)
	}

	doAssertExists(nil, nil)

	p.KluctlMust("deploy", "--yes", "-t", "test")
	doAssertExists(p.listDeploymentItemPathes(".", false), nil)

	p.deleteKustomizeDeployment("cm1")
	p.KluctlMust("prune", "--yes", "-t", "test", "-I", "non-existent-tag")
	doAssertExists(nil, nil)

	p.KluctlMust("prune", "--yes", "-t", "test", "-I", "cm1")
	doAssertExists(nil, []string{"cm1"})

	p.deleteKustomizeDeployment("cm2")
	p.KluctlMust("prune", "--yes", "-t", "test", "-E", "cm2")
	doAssertExists(nil, nil)

	p.deleteKustomizeDeployment("cm3")
	p.KluctlMust("prune", "--yes", "-t", "test", "--exclude-deployment-dir", "cm3")
	doAssertExists(nil, []string{"cm2"})

	p.KluctlMust("prune", "--yes", "-t", "test")
	doAssertExists(nil, []string{"cm3"})
}

func TestInclusionDelete(t *testing.T) {
	t.Parallel()
	p, k := prepareInclusionTestProject(t, "inclusion-delete", false)
	defer p.cleanup()

	shouldExists := make(map[string]bool)
	doAssertExists := func(add []string, remove []string) {
		assertExistsHelper(t, p, k, shouldExists, add, remove)
	}

	p.KluctlMust("deploy", "--yes", "-t", "test")
	doAssertExists(p.listDeploymentItemPathes(".", false), nil)

	p.KluctlMust("delete", "--yes", "-t", "test", "-I", "non-existent-tag")
	doAssertExists(nil, nil)

	p.KluctlMust("delete", "--yes", "-t", "test", "-I", "cm1")
	doAssertExists(nil, []string{"cm1"})

	p.KluctlMust("delete", "--yes", "-t", "test", "-E", "cm2")
	var a []string
	for _, x := range p.listDeploymentItemPathes(".", false) {
		if x != "cm1" && x != "cm2" {
			a = append(a, x)
		}
	}
	doAssertExists(nil, a)
}
