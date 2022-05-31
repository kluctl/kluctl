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

func addDeploymentHelper(p *testProject, dir string, o *uo.UnstructuredObject, opts resourceOpts) {
	rbac := renderTemplateObjectHelper(podRbacTemplate, map[string]interface{}{
		"name":      o.GetK8sName(),
		"namespace": o.GetK8sNamespace(),
	})
	for _, x := range rbac {
		mergeMetadata(x, opts)
	}
	mergeMetadata(o, opts)

	resources := []kustomizeResource{
		{"rbac.yml", "", rbac},
		{"deploy.yml", "", o},
	}

	p.addKustomizeDeployment(dir, resources, opts.tags)
}

func addDeploymentDeployment(p *testProject, dir string, opts resourceOpts, image string, command []string, args []string) {
	o := renderTemplateObjectHelper(deploymentTemplate, map[string]interface{}{
		"name":      opts.name,
		"namespace": opts.namespace,
		"image":     image,
	})
	o[0].SetNestedField(command, "spec", "template", "spec", "containers", 0, "command")
	o[0].SetNestedField(args, "spec", "template", "spec", "containers", 0, "args")
	addDeploymentHelper(p, dir, o[0], opts)
}

func addJobDeployment(p *testProject, dir string, opts resourceOpts, image string, command []string, args []string) {
	o := renderTemplateObjectHelper(jobTemplate, map[string]interface{}{
		"name":      opts.name,
		"namespace": opts.namespace,
		"image":     image,
	})
	o[0].SetNestedField(command, "spec", "template", "spec", "containers", 0, "command")
	o[0].SetNestedField(args, "spec", "template", "spec", "containers", 0, "args")
	addDeploymentHelper(p, dir, o[0], opts)
}

func addBusyboxDeployment(p *testProject, dir string, opts resourceOpts) {
	addDeploymentDeployment(p, dir, opts, "busybox", []string{"sleep"}, []string{"1000"})
}

const podRbacTemplate = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .name }}
  namespace: {{ .namespace }}
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: "{{ .name }}"
  namespace: "{{ .namespace }}"
subjects:
- kind: ServiceAccount
  name: "{{ .name }}"
  namespace: "{{ .namespace }}"
roleRef:
  kind: ClusterRole
  name: "cluster-admin"
  apiGroup: rbac.authorization.k8s.io
`

const deploymentTemplate = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .name }}
  namespace: {{ .namespace }}
  labels:
    app.kubernetes.io/name: {{ .name }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ .name }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ .name }}
    spec:
      terminationGracePeriodSeconds: 0
      serviceAccountName: {{ .name }}
      containers:
        - image: {{ .image }}
          imagePullPolicy: IfNotPresent
          name: container
`

const jobTemplate = `
apiVersion: batch/v1
kind: Job
metadata:
  name: {{ .name }}
  namespace: {{ .namespace }}
spec:
  template:
    spec:
      terminationGracePeriodSeconds: 0
      restartPolicy: OnFailure
      serviceAccountName: {{ .name }}
      containers:
        - image: {{ .image }}
          imagePullPolicy: IfNotPresent
          name: container
`
