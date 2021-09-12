from kluctl.e2e.kluctl_test_project import KluctlTestProject

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
      containers:
      - name: busybox
        image: busybox
        command:
          - sleep
          - "1000"
"""

def add_busybox_deployment(p: KluctlTestProject, dir, name, namespace="default"):
    resources = {
        "busybox.yml": busybox_pod.format(namespace=namespace, name=name)
    }

    p.add_kustomize_deployment(dir, resources)