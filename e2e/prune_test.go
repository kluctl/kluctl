package e2e

import (
	test_utils "github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"testing"
)

func TestPrune(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm1", map[string]string{}, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})
	addConfigMapDeployment(p, "cm2", map[string]string{}, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")

	p.DeleteKustomizeDeployment("cm2")

	p.KluctlMust(t, "prune", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm2")
}

func TestDeployWithPrune(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm1", map[string]string{}, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})
	addConfigMapDeployment(p, "cm2", map[string]string{}, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")

	p.DeleteKustomizeDeployment("cm2")
	addConfigMapDeployment(p, "cm3", map[string]string{}, resourceOpts{
		name:      "cm3",
		namespace: p.TestSlug(),
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test", "--prune")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm2")
	assertConfigMapExists(t, k, p.TestSlug(), "cm3")
}

func TestPruneForceManaged(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", nil)

	addConfigMapDeployment(p, "cm1", map[string]string{}, resourceOpts{
		name:      "cm1",
		namespace: p.TestSlug(),
	})
	addConfigMapDeployment(p, "cm2", map[string]string{}, resourceOpts{
		name:      "cm2",
		namespace: p.TestSlug(),
	})

	p.KluctlMust(t, "deploy", "--yes", "-t", "test")
	cm1 := assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2")

	p.DeleteKustomizeDeployment("cm2")

	patchObject(t, k, v1.SchemeGroupVersion.WithResource("configmaps"), p.TestSlug(), "cm2", func(o *uo.UnstructuredObject) {
		// objects with owner references are not considered when pruning...
		o.SetK8sOwnerReferences([]*uo.UnstructuredObject{
			uo.FromStructMust(&metav1.OwnerReference{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Name:       "cm1",
				UID:        types.UID(cm1.GetK8sUid()),
			}),
		})
	})

	p.KluctlMust(t, "prune", "--yes", "-t", "test")
	assertConfigMapExists(t, k, p.TestSlug(), "cm1")
	assertConfigMapExists(t, k, p.TestSlug(), "cm2") // not pruned because of owner ref

	patchObject(t, k, v1.SchemeGroupVersion.WithResource("configmaps"), p.TestSlug(), "cm2", func(o *uo.UnstructuredObject) {
		// ... except when force-managed=true
		o.SetK8sAnnotation("kluctl.io/force-managed", "true")
	})
	p.KluctlMust(t, "prune", "--yes", "-t", "test")
	assertConfigMapNotExists(t, k, p.TestSlug(), "cm2")
}
