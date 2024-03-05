package e2e

import (
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"testing"
)

func TestDiscriminator(t *testing.T) {
	t.Parallel()

	p := test_project.NewTestProject(t)
	k := defaultCluster1

	addConfigMapDeployment(p, "cm1", nil, resourceOpts{name: "cm1", namespace: p.TestSlug()})

	createNamespace(t, k, p.TestSlug())

	p.UpdateKluctlYaml(func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField("from-kluctl-yaml-{{ args.a }}", "discriminator")
		return nil
	})

	p.KluctlMust(t, "deploy", "--yes", "-a", "a=x")
	cm := assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertNestedFieldEquals(t, cm, "from-kluctl-yaml-x", "metadata", "labels", "kluctl.io/discriminator")

	// add a target without a discriminator
	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test", "-a", "a=x")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertNestedFieldEquals(t, cm, "from-kluctl-yaml-x", "metadata", "labels", "kluctl.io/discriminator")

	// modify target to contain a discriminator
	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField("from-target-{{ target.name }}", "discriminator")
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test", "-a", "a=x")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertNestedFieldEquals(t, cm, "from-target-test", "metadata", "labels", "kluctl.io/discriminator")
}

func TestDiscriminatorArgWithoutTarget(t *testing.T) {
	t.Parallel()

	p := test_project.NewTestProject(t)
	k := defaultCluster1

	addConfigMapDeployment(p, "cm1", nil, resourceOpts{name: "cm1", namespace: p.TestSlug()})
	addConfigMapDeployment(p, "cm2", nil, resourceOpts{name: "cm2", namespace: p.TestSlug()})

	createNamespace(t, k, p.TestSlug())

	p.KluctlMust(t, "deploy", "--yes", "--discriminator", "test-discriminator")

	cm := assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertNestedFieldEquals(t, cm, "test-discriminator", "metadata", "labels", "kluctl.io/discriminator")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "cm2")
	assertNestedFieldEquals(t, cm, "test-discriminator", "metadata", "labels", "kluctl.io/discriminator")

	p.KluctlMust(t, "deploy", "--yes", "--discriminator", "test-discriminator-{{ args.a }}", "-a", "a=x")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertNestedFieldEquals(t, cm, "test-discriminator-x", "metadata", "labels", "kluctl.io/discriminator")

	p.DeleteKustomizeDeployment("cm2")
	p.KluctlMust(t, "prune", "--yes", "--discriminator", "test-discriminator-x")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm2")
}

func TestDiscriminatorArgWithTarget(t *testing.T) {
	t.Parallel()

	p := test_project.NewTestProject(t)
	k := defaultCluster1

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
		_ = target.SetNestedField("from-target-{{ target.name }}", "discriminator")
	})

	addConfigMapDeployment(p, "cm1", nil, resourceOpts{name: "cm1", namespace: p.TestSlug()})
	addConfigMapDeployment(p, "cm2", nil, resourceOpts{name: "cm2", namespace: p.TestSlug()})

	createNamespace(t, k, p.TestSlug())

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm := assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertNestedFieldEquals(t, cm, "from-target-test", "metadata", "labels", "kluctl.io/discriminator")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "cm2")
	assertNestedFieldEquals(t, cm, "from-target-test", "metadata", "labels", "kluctl.io/discriminator")

	p.KluctlMust(t, "deploy", "--yes", "-t", "test", "--discriminator", "test-discriminator")

	cm = assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertNestedFieldEquals(t, cm, "test-discriminator", "metadata", "labels", "kluctl.io/discriminator")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "cm2")
	assertNestedFieldEquals(t, cm, "test-discriminator", "metadata", "labels", "kluctl.io/discriminator")

	p.KluctlMust(t, "deploy", "--yes", "-t", "test", "--discriminator", "test-discriminator-{{ target.name }}-{{ args.a }}", "-a", "a=x")
	cm = assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertNestedFieldEquals(t, cm, "test-discriminator-test-x", "metadata", "labels", "kluctl.io/discriminator")

	p.DeleteKustomizeDeployment("cm3")
	p.KluctlMust(t, "prune", "--yes", "-t", "test", "--discriminator", "test-discriminator-test-x")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm3")
}
