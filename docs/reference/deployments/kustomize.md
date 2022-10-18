---
title: "Kustomize Integration"
linkTitle: "Kustomize Integration"
weight: 2
description: >
    How Kustomize is integrated into Kluctl
---

kluctl uses [kustomize](https://kustomize.io/) to render final resources. This means, that the finest/lowest
level in kluctl is represented with kustomize deployments. These kustomize deployments can then perform further
customization, e.g. patching and more. You can also use kustomize to easily generate ConfigMaps or secrets from files.

Generally, everything is possible via `kustomization.yaml`, is thus possible in kluctl.

We advise to read the kustomize
[reference](https://kubectl.docs.kubernetes.io/references/kustomize/). You can also look into the official kustomize
[example](https://github.com/kubernetes-sigs/kustomize/tree/master/examples).
