<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Readiness"
linkTitle: "Readiness"
weight: 7
description:
  Definition of readiness.
---
-->

# Readiness

There are multiple places where kluctl can wait for "readiness" of resources, e.g. for hooks or when `waitReadiness` is
specified on a deployment item. Readiness depends on the resource kind, e.g. for a Job, kluctl would wait until it
finishes successfully.

## Control via Annotations

Multiple [annotations](./annotations/README.md) control the behaviour when waiting for readiness of resources. These are
the following annoations:

- [kluctl.io/wait-readiness in resources](./annotations/all-resources.md#kluctliowait-readiness)
- [kluctl.io/wait-readiness in kustomization.yaml](./annotations/kustomization.md#kluctliowait-readiness)
- [kluctl.io/is-ready](./annotations/all-resources.md#kluctliois-ready)
- [kluctl.io/hook-wait](./annotations/hooks.md#kluctliohook-wait)
