package e2e

import (
	"github.com/kluctl/kluctl/v2/e2e/test-utils"
	corev1 "k8s.io/api/core/v1"
	"path/filepath"
	"reflect"
	"testing"
)

func prepareInclusionTestProject(t *testing.T, withIncludes bool) (*test_utils.TestProject, *test_utils.EnvTestCluster) {
	k := defaultCluster1
	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm1", nil, resourceOpts{name: "cm1", namespace: p.TestSlug()})
	addConfigMapDeployment(p, "cm2", nil, resourceOpts{name: "cm2", namespace: p.TestSlug()})
	addConfigMapDeployment(p, "cm3", nil, resourceOpts{name: "cm3", namespace: p.TestSlug(), tags: []string{"tag1", "tag2"}})
	addConfigMapDeployment(p, "cm4", nil, resourceOpts{name: "cm4", namespace: p.TestSlug(), tags: []string{"tag1", "tag3"}})
	addConfigMapDeployment(p, "cm5", nil, resourceOpts{name: "cm5", namespace: p.TestSlug(), tags: []string{"tag1", "tag4"}})
	addConfigMapDeployment(p, "cm6", nil, resourceOpts{name: "cm6", namespace: p.TestSlug(), tags: []string{"tag1", "tag5"}})
	addConfigMapDeployment(p, "cm7", nil, resourceOpts{name: "cm7", namespace: p.TestSlug(), tags: []string{"tag1", "tag6"}})

	if withIncludes {
		p.AddDeploymentInclude(".", "include1", nil)
		addConfigMapDeployment(p, "include1/icm1", nil, resourceOpts{name: "icm1", namespace: p.TestSlug(), tags: []string{"itag1", "itag2"}})

		p.AddDeploymentInclude(".", "include2", nil)
		addConfigMapDeployment(p, "include2/icm2", nil, resourceOpts{name: "icm2", namespace: p.TestSlug()})
		addConfigMapDeployment(p, "include2/icm3", nil, resourceOpts{name: "icm3", namespace: p.TestSlug(), tags: []string{"itag3", "itag4"}})

		p.AddDeploymentInclude(".", "include3", []string{"itag5"})
		addConfigMapDeployment(p, "include3/icm4", nil, resourceOpts{name: "icm4", namespace: p.TestSlug()})
		addConfigMapDeployment(p, "include3/icm5", nil, resourceOpts{name: "icm5", namespace: p.TestSlug(), tags: []string{"itag5", "itag6"}})
	}

	return p, k
}

func assertExistsHelper(t *testing.T, p *test_utils.TestProject, k *test_utils.EnvTestCluster, shouldExists map[string]bool, add []string, remove []string) {
	for _, x := range add {
		shouldExists[x] = true
	}
	for _, x := range remove {
		if _, ok := shouldExists[x]; ok {
			delete(shouldExists, x)
		}
	}
	items, err := k.List(corev1.SchemeGroupVersion.WithResource("configmaps"), p.TestSlug(), map[string]string{"project_name": p.TestSlug()})
	if err != nil {
		t.Fatal(err)
	}
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
	p, k := prepareInclusionTestProject(t, false)

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
	p, k := prepareInclusionTestProject(t, false)

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
	p, k := prepareInclusionTestProject(t, true)

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
	p, k := prepareInclusionTestProject(t, true)

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
	for _, x := range p.ListDeploymentItemPathes(".", false) {
		if x != "icm5" {
			a = append(a, filepath.Base(x))
		}
	}
	doAssertExists(a...)
}

func TestInclusionPrune(t *testing.T) {
	t.Parallel()
	p, k := prepareInclusionTestProject(t, false)

	shouldExists := make(map[string]bool)
	doAssertExists := func(add []string, remove []string) {
		assertExistsHelper(t, p, k, shouldExists, add, remove)
	}

	doAssertExists(nil, nil)

	p.KluctlMust("deploy", "--yes", "-t", "test")
	doAssertExists(p.ListDeploymentItemPathes(".", false), nil)

	p.DeleteKustomizeDeployment("cm1")
	p.KluctlMust("prune", "--yes", "-t", "test", "-I", "non-existent-tag")
	doAssertExists(nil, nil)

	p.KluctlMust("prune", "--yes", "-t", "test", "-I", "cm1")
	doAssertExists(nil, []string{"cm1"})

	p.DeleteKustomizeDeployment("cm2")
	p.KluctlMust("prune", "--yes", "-t", "test", "-E", "cm2")
	doAssertExists(nil, nil)

	p.DeleteKustomizeDeployment("cm3")
	p.KluctlMust("prune", "--yes", "-t", "test", "--exclude-deployment-dir", "cm3")
	doAssertExists(nil, []string{"cm2"})

	p.KluctlMust("prune", "--yes", "-t", "test")
	doAssertExists(nil, []string{"cm3"})
}

func TestInclusionDelete(t *testing.T) {
	t.Parallel()
	p, k := prepareInclusionTestProject(t, false)

	shouldExists := make(map[string]bool)
	doAssertExists := func(add []string, remove []string) {
		assertExistsHelper(t, p, k, shouldExists, add, remove)
	}

	p.KluctlMust("deploy", "--yes", "-t", "test")
	doAssertExists(p.ListDeploymentItemPathes(".", false), nil)

	p.KluctlMust("delete", "--yes", "-t", "test", "-I", "non-existent-tag")
	doAssertExists(nil, nil)

	p.KluctlMust("delete", "--yes", "-t", "test", "-I", "cm1")
	doAssertExists(nil, []string{"cm1"})

	p.KluctlMust("delete", "--yes", "-t", "test", "-E", "cm2")
	var a []string
	for _, x := range p.ListDeploymentItemPathes(".", false) {
		if x != "cm1" && x != "cm2" {
			a = append(a, x)
		}
	}
	doAssertExists(nil, a)
}
