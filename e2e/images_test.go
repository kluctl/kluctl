package e2e

import (
	"fmt"
	test_utils "github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"testing"
)

func addGetImageDeployment(p *test_utils.TestProject, name string, containerName string, gi string) {
	y := fmt.Sprintf(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: %s
  namespace: %s
  labels:
    app: %s
spec:
  replicas: 1
  selector:
    matchLabels:
      app: %s
  template:
    metadata:
      labels:
        app: %s
    spec:
      containers:
      - name: %s
        image: '%s'
`, name, p.TestSlug(), name, name, name, containerName, gi)

	p.AddKustomizeDeployment(name, []test_utils.KustomizeResource{
		{Name: name, Content: uo.FromStringMust(y)},
	}, nil)
}

func assertImage(t *testing.T, k *test_utils.EnvTestCluster, p *test_utils.TestProject, deploymentName string, containerName string, expectedImage string) {
	d := assertObjectExists(t, k, schema.GroupVersionResource{
		Group:    "apps",
		Version:  "v1",
		Resource: "deployments",
	}, p.TestSlug(), deploymentName)

	l, _, _ := d.GetNestedObjectList("spec", "template", "spec", "containers")
	for _, x := range l {
		n, _, _ := x.GetNestedString("name")
		if n != containerName {
			continue
		}
		image, _, _ := x.GetNestedString("image")
		assert.Equal(t, expectedImage, image)
		return
	}

	assert.Fail(t, fmt.Sprintf("container %s not found", containerName))
}

func TestGetImageNotFound(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
	})

	addGetImageDeployment(p, "d1", "c1", `{{ images.get_image("i1") }}`)

	_, stderr, err := p.Kluctl("deploy", "-y", "-t", "test")
	assert.Error(t, err)
	assert.Contains(t, stderr, "failed to find fixed image for i1")
}

func TestGetImageArg(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
	})

	addGetImageDeployment(p, "d1", "c1", `{{ images.get_image("i1") }}`)

	p.KluctlMust("deploy", "-y", "-t", "test", "--fixed-image", "i1=i1:arg")
	assertImage(t, k, p, "d1", "c1", "i1:arg")
}

func setImagesVars(p *test_utils.TestProject, fis []types.FixedImage) {
	vars := []types.VarsSource{
		{
			Values: uo.FromMap(map[string]interface{}{
				"images": fis,
			}),
		},
	}

	p.UpdateDeploymentYaml(".", func(o *uo.UnstructuredObject) error {
		_ = o.SetNestedField(vars, "vars")
		return nil
	})
}

func TestGetImageVars(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	setImagesVars(p, []types.FixedImage{
		{Image: utils.StrPtr("i1"), ResultImage: "i1:vars"},
	})

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
	})

	addGetImageDeployment(p, "d1", "c1", `{{ images.get_image("i1") }}`)

	p.KluctlMust("deploy", "-y", "-t", "test")
	assertImage(t, k, p, "d1", "c1", "i1:vars")
}

func TestGetImageMixed(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	setImagesVars(p, []types.FixedImage{
		{Image: utils.StrPtr("i2"), ResultImage: "i2:vars"},
	})

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
	})

	addGetImageDeployment(p, "d1", "c1", `{{ images.get_image("i1") }}`)
	addGetImageDeployment(p, "d2", "c2", `{{ images.get_image("i2") }}`)

	p.KluctlMust("deploy", "-y", "-t", "test", "--fixed-image", "i1=i1:arg")
	assertImage(t, k, p, "d1", "c1", "i1:arg")
	assertImage(t, k, p, "d2", "c2", "i2:vars")
}

func TestGetImageByDeployment(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	setImagesVars(p, []types.FixedImage{
		{Image: utils.StrPtr("i1"), ResultImage: "i1:vars1", Deployment: utils.StrPtr("Deployment/d1")},
		{Image: utils.StrPtr("i1"), ResultImage: "i1:vars2", Deployment: utils.StrPtr("Deployment/d2")},
	})

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
	})

	addGetImageDeployment(p, "d1", "c1", `{{ images.get_image("i1") }}`)
	addGetImageDeployment(p, "d2", "c2", `{{ images.get_image("i1") }}`)

	p.KluctlMust("deploy", "-y", "-t", "test")
	assertImage(t, k, p, "d1", "c1", "i1:vars1")
	assertImage(t, k, p, "d2", "c2", "i1:vars2")
}

func TestGetImageByContainer(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	setImagesVars(p, []types.FixedImage{
		{Image: utils.StrPtr("i1"), ResultImage: "i1:vars1", Container: utils.StrPtr("c1")},
		{Image: utils.StrPtr("i1"), ResultImage: "i1:vars2", Container: utils.StrPtr("c2")},
	})

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
	})

	addGetImageDeployment(p, "d1", "c1", `{{ images.get_image("i1") }}`)
	addGetImageDeployment(p, "d2", "c2", `{{ images.get_image("i1") }}`)

	p.KluctlMust("deploy", "-y", "-t", "test")
	assertImage(t, k, p, "d1", "c1", "i1:vars1")
	assertImage(t, k, p, "d2", "c2", "i1:vars2")
}

func TestGetImageRegex(t *testing.T) {
	t.Parallel()

	k := defaultCluster1

	p := test_utils.NewTestProject(t)

	setImagesVars(p, []types.FixedImage{
		{ImageRegex: utils.StrPtr("i.*"), ResultImage: "i1:x"},
		{ImageRegex: utils.StrPtr("j.*"), ResultImage: "i1:y"},
	})

	createNamespace(t, k, p.TestSlug())

	p.UpdateTarget("test", func(target *uo.UnstructuredObject) {
	})

	addGetImageDeployment(p, "d1", "c1", `{{ images.get_image("i1") }}`)
	addGetImageDeployment(p, "d2", "c2", `{{ images.get_image("i2") }}`)
	addGetImageDeployment(p, "d3", "c1", `{{ images.get_image("j1") }}`)
	addGetImageDeployment(p, "d4", "c2", `{{ images.get_image("j2") }}`)

	p.KluctlMust("deploy", "-y", "-t", "test")
	assertImage(t, k, p, "d1", "c1", "i1:x")
	assertImage(t, k, p, "d2", "c2", "i1:x")
	assertImage(t, k, p, "d3", "c1", "i1:y")
	assertImage(t, k, p, "d4", "c2", "i1:y")
}
