from kluctl.e2e.kluctl_test_project import KluctlTestProject
from kluctl.utils.yaml_utils import yaml_dump

busybox_pod = """
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {name}
  labels:
    app: {name}
  namespace: {namespace}
spec:
  replicas: 1
  selector:
    matchLabels:
      app: {name}
  template:
    metadata:
      labels:
        app: {name}
    spec:
      terminationGracePeriodSeconds: 0
      containers:
      - name: busybox
        image: busybox
        command:
          - sleep
          - "1000"
"""

def add_busybox_deployment(p: KluctlTestProject, dir, name, namespace="default"):
    p.add_kustomize_deployment(dir, {
        "busybox.yml": busybox_pod.format(namespace=namespace, name=name)
    })

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

def add_configmap_deployment(p: KluctlTestProject, dir, name, namespace="default", data={}):
    y = {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {
            "name": name,
            "namespace": namespace,
        },
        "data": data,
    }
    p.add_kustomize_deployment(dir, {
        "configmap.yml": yaml_dump(y),
    })
