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
