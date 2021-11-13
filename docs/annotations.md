# Annotations

kluctl supports multiple annotations that influence individual commands. These are:

## Deployment related
These annotations control details about how deployments should be handled.

### kluctl.io/skip-delete
If set to "true", the annotated resource will not be deleted when [delete](./commands.md#delete) or 
[prune](./commands.md#prune) is called.

### kluctl.io/skip-delete-if-tags
If set to "true", the annotated resource will not be deleted when [delete](./commands.md#delete) or 
[prune](./commands.md#prune) is called and inclusion/exclusion tags are used at the same time.

This tag is especially useful and required on resources that would otherwise cause cascaded deletions of resources that
do not match the specified inclusion/exclusion tags. Namespaces are the most prominent example of such resources, as
they most likely don't match exclusion tags, but cascaded deletion would still cause deletion of the excluded resources.

### kluctl.io/downscale-patch
Describes how a [downscale](./commands.md#downscale) on a resource can be done via a json patch. This is useful on
CRD based resources where no automatic downscale can be performed by kluctl.

Example:
```yaml
apiVersion: kibana.k8s.elastic.co/v1
kind: Kibana
metadata:
  name: kibana
  annotations:
    kluctl.io/downscale-patch: |
      - op: replace
        path: /spec/count
        value: 0
spec:
  version: 7.14.1
  count: 1
```

If more than one patch needs to be specified, add `-xxx` to the annoation key, where `xxx` is an arbitrary number.

### kluctl.io/downscale-ignore
If set to "true", the resource will be ignored while [downscale](./commands.md#downscale) is executed.

## Hooks related
See [hooks](./hooks.md) for more details.

### kluctl.io/hook
Declares a resource to be a hook, which is deployed/executed as described in [hooks](./hooks.md). The value of the
annotation determines when the hook is deployed/executed.

### kluctl.io/hook-weight
Specifies a weight for the hook, used to determine deployment/execution order.

### kluctl.io/hook-delete-policy
Defines when to delete the hook resource.

### kluctl.io/hook-wait
Defines whether kluctl should wait for hook-completion.

## Validation related
The following annotations influence the [validate](./commands.md#validate) command.

### validate-result.kluctl.io/xxx
If this annotation is found on a resource that is checked while validation, the key and the value of the annotation
are added to the validation result, which is then returned by the validate command.

The annotation key is dynamic, meaning that all annotations that begin with `validate-result.kluctl.io/` are taken
into account.

