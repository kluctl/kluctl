# Hooks

kluctl supports hooks in a similar fashion as known from Helm Charts. Hooks are executed/deployed before and/or after the
actual deployment of a kustomize deployment.

To mark a resource as a hook, add the `kluctl.io/hook` annotation to a resource. The value of the annotation must be
a comma separated list of hook names. Possible value are described in the next chapter.

## Hook types

| Hook Type | Description |
|---|---|
| pre-deploy-initial | Executed right before the initial deployment is performed.<br>See below for a description on what "initial" means |
| post-deploy-initial | Executed right after the initial deployment is performed.<br>See below for a description on what "initial" means |
| pre-deploy | Executed right before a deployment is performed.<br>This also includes the initial deployment |
| post-deploy | Executed right after a deployment is performed.<br>This also includes the initial deployment |

A deployment is considered to be an "initial" deployment if none of the resources related to the current kustomize
deployment are found on the cluster at the time of deployment.

pre-deploy and post-deploy are ALWAYS executed, not considering the "initial" state of the deployment.

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
waits for the hook resources to become "ready". Readiness depends on the resource kind, e.g. for a Job, kluctl would
wait until it finishes successfully.
