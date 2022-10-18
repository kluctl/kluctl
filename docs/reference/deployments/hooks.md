---
title: "Hooks"
linkTitle: "Hooks"
weight: 4
description: >
    Kluctl hooks.
---

Kluctl supports hooks in a similar fashion as known from Helm Charts. Hooks are executed/deployed before and/or after the
actual deployment of a kustomize deployment.

To mark a resource as a hook, add the `kluctl.io/hook` annotation to a resource. The value of the annotation must be
a comma separated list of hook names. Possible value are described in the next chapter.

## Hook types

| Hook Type | Description |
|---|---|
| pre-deploy-initial | Executed right before the initial deployment is performed. |
| post-deploy-initial | Executed right after the initial deployment is performed. |
| pre-deploy-upgrade | Executed right before a non-initial deployment is performed. |
| post-deploy-upgrade | Executed right after a non-initial deployment is performed. |
| pre-deploy | Executed right before any (initial and non-initial) deployment is performed.|
| post-deploy | Executed right after any (initial and non-initial) deployment is performed. |

A deployment is considered to be an "initial" deployment if none of the resources related to the current kustomize
deployment are found on the cluster at the time of deployment.

If you need to execute hooks for every deployment, independent of its "initial" state, use
`pre-deploy-initial,pre-deploy` to indicate that it should be executed all the time.

## Hook deletion

Hook resources are by default deleted right before creation (if they already existed before). This behavior can be
changed by setting the `kluctl.io/hook-delete-policy` to a comma separated list of the following values:

| Policy | Description |
|---|---|
| before-hook-creation | The default behavior, which means that the hook resource is deleted right before (re-)creation. |
| hook-succeeded | Delete the hook resource directly after it got "ready" |
| hook-failed | Delete the hook resource when it failed to get "ready" |

## Hook readiness

After each deployment/execution of the hooks that belong to a deployment stage (before/after deployment), kluctl
waits for the hook resources to become "ready". Readiness is defined [here]({{< ref "./readiness" >}}).

It is possible to disable waiting for hook readiness by setting the annotation `kluctl.io/hook-wait` to "false".
