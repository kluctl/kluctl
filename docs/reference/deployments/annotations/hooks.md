---
title: "Hooks"
linkTitle: "Hooks"
weight: 2
description: >
  Annotations on hooks
---
The following annotations control hook execution

See [hooks]({{< ref "docs/reference/deployments/hooks" >}}) for more details.

### kluctl.io/hook
Declares a resource to be a hook, which is deployed/executed as described in [hooks]({{< ref "docs/reference/deployments/hooks" >}}). The value of the
annotation determines when the hook is deployed/executed.

### kluctl.io/hook-weight
Specifies a weight for the hook, used to determine deployment/execution order.

### kluctl.io/hook-delete-policy
Defines when to delete the hook resource.

### kluctl.io/hook-wait
Defines whether kluctl should wait for hook-completion.
