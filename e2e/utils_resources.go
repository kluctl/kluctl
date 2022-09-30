package e2e

import (
	"bytes"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"text/template"
)

type resourceOpts struct {
	name        string
	namespace   string
	tags        []string
	labels      map[string]string
	annotations map[string]string
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

func renderTemplateHelper(tmpl string, m map[string]interface{}) string {
	t := template.Must(template.New("").Parse(tmpl))
	r := bytes.NewBuffer(nil)
	err := t.Execute(r, m)
	if err != nil {
		panic(err)
	}
	return r.String()
}

func renderTemplateObjectHelper(tmpl string, m map[string]interface{}) []*uo.UnstructuredObject {
	s := renderTemplateHelper(tmpl, m)
	ret, err := uo.FromStringMulti(s)
	if err != nil {
		panic(err)
	}
	return ret
}

func addConfigMapDeployment(p *testProject, dir string, data map[string]string, opts resourceOpts) {
	o := uo.New()
	o.SetK8sGVKs("", "v1", "ConfigMap")
	mergeMetadata(o, opts)
	if data != nil {
		o.SetNestedField(data, "data")
	}
	p.addKustomizeDeployment(dir, []kustomizeResource{
		{fmt.Sprintf("configmap-%s.yml", opts.name), "", o},
	}, opts.tags)
}

func addSecretDeployment(p *testProject, dir string, data map[string]string, sealedSecret bool, opts resourceOpts) {
	o := uo.New()
	o.SetK8sGVKs("", "v1", "Secret")
	mergeMetadata(o, opts)
	if data != nil {
		o.SetNestedField(data, "stringData")
	}
	fname := fmt.Sprintf("secret-%s.yml", opts.name)
	p.addKustomizeDeployment(dir, []kustomizeResource{
		{fname, fname + ".sealme", o},
	}, opts.tags)
}
