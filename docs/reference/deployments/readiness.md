---
title: "Readiness"
linkTitle: "Readiness"
weight: 5
description:
  Definition of readiness.
---

There are multiple places where kluctl can wait for "readiness" of resources, e.g. for hooks or when `waitReadiness` is
specified on a deployment item. Readiness depends on the resource kind, e.g. for a Job, kluctl would wait until it
finishes successfully.

After each deployment/execution of the hooks that belong to a deployment stage (before/after deployment), kluctl
waits for the hook resources to become "ready". Readiness depends on the resource kind, e.g. for a Job, kluctl would
wait until it finishes successfully.
