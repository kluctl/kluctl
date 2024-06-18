package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test_project"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"path/filepath"
)

type resourceOpts struct {
	name        string
	fname       string
	namespace   string
	tags        []string
	labels      map[string]string
	annotations map[string]string
	when        string
}

func mergeMetadata(o *uo.UnstructuredObject, opts resourceOpts) {
	if opts.name != "" {
		o.SetK8sName(opts.name)
	}
	if opts.namespace != "" {
		o.SetK8sNamespace(opts.namespace)
	}
	if opts.labels != nil {
		o.SetK8sLabels(opts.labels)
	}
	if opts.annotations != nil {
		o.SetK8sAnnotations(opts.annotations)
	}
}

func createCoreV1Object(kind string, opts resourceOpts) *uo.UnstructuredObject {
	o := uo.New()
	o.SetK8sGVKs("", "v1", kind)
	mergeMetadata(o, opts)
	return o
}

func createConfigMapObject(data map[string]string, opts resourceOpts) *uo.UnstructuredObject {
	o := createCoreV1Object("ConfigMap", opts)
	if data != nil {
		o.SetNestedField(data, "data")
	}
	return o
}

func createSecretObject(data map[string]string, opts resourceOpts) *uo.UnstructuredObject {
	o := createCoreV1Object("Secret", opts)
	if data != nil {
		o.SetNestedField(data, "stringData")
	}
	return o
}

func addConfigMapDeployment(p *test_project.TestProject, dir string, data map[string]string, opts resourceOpts) {
	o := createConfigMapObject(data, opts)
	fname := opts.fname
	if fname == "" {
		fname = fmt.Sprintf("configmap-%s.yml", opts.name)
	}
	p.AddKustomizeDeployment(dir, []test_project.KustomizeResource{
		{Name: fname, Content: o},
	}, opts.tags)
	if opts.when != "" {
		p.UpdateDeploymentItems(filepath.Dir(dir), func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
			_ = items[len(items)-1].SetNestedField(opts.when, "when")
			return items
		})
	}
}

func addSecretDeployment(p *test_project.TestProject, dir string, data map[string]string, opts resourceOpts) {
	o := createSecretObject(data, opts)
	fname := fmt.Sprintf("secret-%s.yml", opts.name)
	p.AddKustomizeDeployment(dir, []test_project.KustomizeResource{
		{Name: fname, FileName: fname, Content: o},
	}, opts.tags)
	if opts.when != "" {
		p.UpdateDeploymentItems(filepath.Dir(dir), func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
			_ = items[len(items)-1].SetNestedField(opts.when, "when")
			return items
		})
	}
}
