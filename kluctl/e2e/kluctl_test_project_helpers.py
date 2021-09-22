from kluctl.e2e.kluctl_test_project import KluctlTestProject
from kluctl.utils.dict_utils import merge_dict, set_dict_value
from kluctl.utils.yaml_utils import yaml_dump, yaml_load, yaml_load_all, yaml_dump_all


def merge_metadata(y, labels, annotations):
    if labels is not None:
        merge_dict(y, {
            "metadata": {
                "labels": labels,
            }
        }, clone=False)
    if annotations is not None:
        merge_dict(y, {
            "metadata": {
                "annotations": annotations,
            }
        }, clone=False)

pod_rbac="""
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {name}
  namespace: {namespace}
---
kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: "{name}"
  namespace: "{namespace}"
subjects:
- kind: ServiceAccount
  name: "{name}"
  namespace: "{namespace}"
roleRef:
  kind: ClusterRole
  name: "cluster-admin"
  apiGroup: rbac.authorization.k8s.io
"""

pod="""
apiVersion: v1
kind: Pod
metadata:
  name: {name}
  namespace: {namespace}
spec:
  terminationGracePeriodSeconds: 0
  restartPolicy: OnFailure
  serviceAccountName: {name}
  containers:
    - image: {image}
      imagePullPolicy: IfNotPresent
      name: container
"""

job="""
apiVersion: batch/v1
kind: Job
metadata:
  name: {name}
  namespace: {namespace}
spec:
  template:
    spec:
      terminationGracePeriodSeconds: 0
      restartPolicy: OnFailure
      serviceAccountName: {name}
      containers:
        - image: {image}
          imagePullPolicy: IfNotPresent
          name: container
"""


def add_deployment_helper(p: KluctlTestProject, dir, name,
                          r, with_rbac,
                          namespace="default", tags=None,
                          labels=None, annotations=None):
    resources = {}

    if with_rbac:
        y = pod_rbac.format(name=name, namespace=namespace)
        y = yaml_load_all(y)
        for x in y:
            merge_metadata(x, labels, annotations)
        resources["rbac.yml"] = yaml_dump_all(y)

    merge_metadata(r, labels, annotations)
    resources["deploy.yml"] = yaml_dump(r)

    p.add_kustomize_deployment(dir, resources, tags=tags)

def add_pod_deployment(p: KluctlTestProject, dir, name,
                       image, command, args, with_rbac,
                       namespace="default", **kwargs):
    y = pod.format(name=name, namespace=namespace, image=image)
    y = yaml_load(y)
    set_dict_value(y, "spec.containers[0].command", command)
    set_dict_value(y, "spec.containers[0].args", args)
    add_deployment_helper(p, dir, name, y, with_rbac, namespace=namespace, **kwargs)

def add_job_deployment(p: KluctlTestProject, dir, name,
                       image, command, args, with_rbac,
                       namespace="default", **kwargs):
    y = job.format(name=name, namespace=namespace, image=image)
    y = yaml_load(y)
    set_dict_value(y, "spec.template.spec.containers[0].command", command)
    set_dict_value(y, "spec.template.spec.containers[0].args", args)
    add_deployment_helper(p, dir, name, y, with_rbac, namespace=namespace, **kwargs)

def add_busybox_deployment(p: KluctlTestProject, dir, name,
                           namespace="default"):
    add_pod_deployment(p, dir, name, "busybox", "sleep", ["1000"], False, namespace=namespace)

def add_namespace_deployment(p: KluctlTestProject, dir, name):
    y = {
        "apiVersion": "v1",
        "kind": "Namespace",
        "metadata": {
            "name": name,
        },
    }
    p.add_kustomize_deployment(dir, {
        "namespace.yml": yaml_dump(y),
    })

def add_configmap_deployment(p: KluctlTestProject, dir, name, namespace="default", data={}, tags=None, labels=None, annotations=None):
    y = {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {
            "name": name,
            "namespace": namespace,
        },
        "data": data,
    }
    merge_metadata(y, labels, annotations)
    p.add_kustomize_deployment(dir, {
        "configmap.yml": yaml_dump(y),
    }, tags=tags)
