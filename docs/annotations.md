# Annotations on object level

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

### kluctl.io/diff-name
This annotation will override the name of the object when looking for the in-cluster version of an object used for
diffs. This is useful when you are forced to use new names for the same objects whenever the content changes, e.g.
for all kinds of immutable resource types.

Example (filename job.yml):
```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: myjob-{{ load_sha256("job.yml", 6) }}
  annotations:
    kluctl.io/diff-name: myjob
spec:
  template:
    spec:
      containers:
      - name: hello
        image: busybox
        command: ["sh",  "-c", "echo hello"]
      restartPolicy: Never
```

Without the `kluctl.io/diff-name` annotation, any change to the `job.yml` would be treated as a new object in resulting
diffs from various commands. This is due to the inclusion of the file hash in the job name. This would make it very hard
to figure out what exactly changed in an object.

With the `kluctl.io/diff-name` annotation, kluctl will pick an existing job from the cluster with the same diff-name
and use it for the diff, making it a lot easier to analyze changes. If multiple objects match, the one with the youngest
`creationTimestamp` is chosen.

Please note that this will not cause old objects (with the same diff-name) to be prunes. You still have to regularely
prune the deployment.

### kluctl.io/downscale-patch
Describes how a [downscale](./commands.md#downscale) on a resource can be done via a json patch. This is useful on
CRD based resources where no automatic downscale can be performed by kluctl.

Example (filename job.yml):
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

If more than one patch needs to be specified, add `-xxx` to the annotation key, where `xxx` is an arbitrary number.

### kluctl.io/downscale-ignore
If set to "true", the resource will be ignored while [downscale](./commands.md#downscale) is executed.

### kluctl.io/downscale-delete
If set to "true", the resource will be deleted while [downscale](./commands.md#downscale) is executed.

### kluctl.io/force-apply
If set to "true", the whole resource will be force-applied, meaning that all fields will be overwritten in case of
field manager conflicts.

### kluctl.io/force-apply-field
Specifies a [JSON Path](https://goessner.net/articles/JsonPath/) for fields that should be force-applied. Matching
fields will be overwritten in case of field manager conflicts.

If more than one field needs to be specified, add `-xxx` to the annotation key, where `xxx` is an arbitrary number.

### kluctl.io/ignore-diff
If set to "true", the whole resource will be ignored while calculating diffs.

### kluctl.io/ignore-diff-field
Specifies a [JSON Path](https://goessner.net/articles/JsonPath/) for fields that should be ignored while calculating
diffs.

If more than one field needs to be specified, add `-xxx` to the annotation key, where `xxx` is an arbitrary number.

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

# Annotations on kustomize deployment level

In addition to kluctl.io annotations which can be set on object/resource level, you can also set a few annotations
inside the kustomization.yml itself.

Example:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

metadata:
  annotations:
    kluctl.io/barrier: "true"
    kluctl.io/wait-readiness: "true"

resources:
  - deployment.yml
```

### kluctl.io/barrier
If set to `true`, kluctl will wait for all previous objects to be applied (but not necessarily ready). This has the
same effect as [barrier](./deployments.md#barriers) from deployment projects.

### kluctl.io/wait-readiness
If set to `true`, kluctl will wait for readiness of all objects from this kustomization project. Readiness is defined
the same as in [hook readiness](./hooks.md#hook-readiness).
