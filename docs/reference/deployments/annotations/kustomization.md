---
title: "Kustomize"
linkTitle: "Kustomize"
weight: 4
description: >
  Annotations on the kustomization.yaml resource
---

Even though the `kustomization.yaml` from Kustomize deployments are not really Kubernetes resources (as they are not
really deployed), they have the same structure as Kubernetes resources. This also means that the `kustomization.yaml`
can define metadata and annotations. Through these annotations, additional behavior on the deployment can be controlled.

Example:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

metadata:
  annotations:
    kluctl.io/barrier: "true"
    kluctl.io/wait-readiness: "true"

resources:
  - deployment.yaml
```

### kluctl.io/barrier
If set to `true`, kluctl will wait for all previous objects to be applied (but not necessarily ready). This has the
same effect as [barrier]({{< ref "docs/reference/deployments#barriers" >}}) from deployment projects.

### kluctl.io/wait-readiness
If set to `true`, kluctl will wait for readiness of all objects from this kustomization project. Readiness is defined
the same as in [hook readiness]({{< ref "docs/reference/deployments/readiness" >}}).
