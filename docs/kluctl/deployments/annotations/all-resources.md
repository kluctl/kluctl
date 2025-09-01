<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "All resources"
linkTitle: "All resources"
weight: 1
description: >
  Annotations on all resources
---
-->

# All resources

The following annotations control the behavior of the `deploy` and related commands.

## Control deploy behavior

The following annotations control deploy behavior, especially in regard to conflict resolution.

### kluctl.io/ignore
If set to "true", the resource is completely ignored by Kluctl. This is equivalent to the resource not existing in your
project.

This however means that if it was deployed in the past and then later `kluctl.io/ignore` is set to "true", the resource
will be treated as orphan (which will cause deletion on prune). To avoid this, also set `kluctl.io/skip-delete` to "true".

### kluctl.io/delete
If set to "true", the resource will be deleted at deployment time. Kluctl will not emit an error in case the resource
does not exist. A resource with this annotation does not have to be complete/valid as it is never sent to the Kubernetes
api server.

### kluctl.io/force-apply
If set to "true", the whole resource will be force-applied, meaning that all fields will be overwritten in case of
field manager conflicts.

As an alternative, conflict resolution can be controlled via [conflictResolution](../deployment-yml.md#conflictresolution).

### kluctl.io/force-apply-field
Specifies a [JSON Path](https://goessner.net/articles/JsonPath/) for fields that should be force-applied. Matching
fields will be overwritten in case of field manager conflicts.

If more than one field needs to be specified, add `-xxx` to the annotation key, where `xxx` is an arbitrary number.

As an alternative, conflict resolution can be controlled via [conflictResolution](../deployment-yml.md#conflictresolution).

### kluctl.io/force-apply-manager
Specifies a regex for managers that should be force-applied. Fields with matching managers will be overwritten in
case of field manager conflicts.

If more than one field needs to be specified, add `-xxx` to the annotation key, where `xxx` is an arbitrary number.

As an alternative, conflict resolution can be controlled via [conflictResolution](../deployment-yml.md#conflictresolution).

### kluctl.io/ignore-conflicts
If set to "true", the whole all fields of the object are going to be ignored when conflicts arise.
This effectively disables the warnings that are shown when field ownership is lost.

As an alternative, conflict resolution can be controlled via [conflictResolution](../deployment-yml.md#conflictresolution).

### kluctl.io/ignore-conflicts-field
Specifies a [JSON Path](https://goessner.net/articles/JsonPath/) for fields that should be ignored when conflicts arise.
This effectively disables the warnings that are shown when field ownership is lost.

If more than one field needs to be specified, add `-xxx` to the annotation key, where `xxx` is an arbitrary number.

As an alternative, conflict resolution can be controlled via [conflictResolution](../deployment-yml.md#conflictresolution).

### kluctl.io/ignore-conflicts-manager
Specifies a regex for field managers that should be ignored when conflicts arise.
This effectively disables the warnings that are shown when field ownership is lost.

If more than one manager needs to be specified, add `-xxx` to the annotation key, where `xxx` is an arbitrary number.

As an alternative, conflict resolution can be controlled via [conflictResolution](../deployment-yml.md#conflictresolution).

### kluctl.io/wait-readiness
If set to `true`, kluctl will wait for readiness of this object. Readiness is defined
the same as in [hook readiness](../../deployments/readiness.md). Waiting happens after all resources from the parent 
deployment item have been applied.

### kluctl.io/is-ready
If set to `true`, kluctl will always consider this object as [ready](../../deployments/readiness.md). If set to `false`,
kluctl will always consider this object as not ready. If omitted, kluctl will perform normal readiness checks.

This annotation is useful if you need to introduce externalized readiness determination, e.g. inside a non-hook `Pod`
that can annotate an object that something got ready.

## Control deletion/pruning

The following annotations control how delete/prune is behaving.

### kluctl.io/skip-delete
If set to "true", the annotated resource will not be deleted when [delete](../../commands/delete.md) or
[prune](../../commands/prune.md) is called.

### kluctl.io/skip-delete-if-tags
If set to "true", the annotated resource will not be deleted when [delete](../../commands/delete.md) or
[prune](../../commands/prune.md) is called and inclusion/exclusion tags are used at the same time.

This tag is especially useful and required on resources that would otherwise cause cascaded deletions of resources that
do not match the specified inclusion/exclusion tags. Namespaces are the most prominent example of such resources, as
they most likely don't match exclusion tags, but cascaded deletion would still cause deletion of the excluded resources.

### kluctl.io/force-managed
If set to "true", Kluctl will always treat the annotated resource as being managed by Kluctl, meaning that it will
consider it for deletion and pruning even if a foreign field manager resets/removes the Kluctl field manager or if
foreign controllers add `ownerReferences` even though they do not really own the resources.

## Control diff behavior

The following annotations control how diffs are performed.

### kluctl.io/diff-name
This annotation will override the name of the object when looking for the in-cluster version of an object used for
diffs. This is useful when you are forced to use new names for the same objects whenever the content changes, e.g.
for all kinds of immutable resource types.

Example (filename job.yaml):
```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: myjob-{{ load_sha256("job.yaml", 6) }}
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

Without the `kluctl.io/diff-name` annotation, any change to the `job.yaml` would be treated as a new object in resulting
diffs from various commands. This is due to the inclusion of the file hash in the job name. This would make it very hard
to figure out what exactly changed in an object.

With the `kluctl.io/diff-name` annotation, kluctl will pick an existing job from the cluster with the same diff-name
and use it for the diff, making it a lot easier to analyze changes. If multiple objects match, the one with the youngest
`creationTimestamp` is chosen.

Please note that this will not cause old objects (with the same diff-name) to be prunes. You still have to regularely
prune the deployment.

### kluctl.io/ignore-diff
If set to "true", the whole resource will be ignored while calculating diffs.

### kluctl.io/ignore-diff-field
Specifies a [JSON Path](https://goessner.net/articles/JsonPath/) for fields that should be ignored while calculating
diffs.

If more than one field needs to be specified, add `-xxx` to the annotation key, where `xxx` is an arbitrary number.

### kluctl.io/ignore-diff-field-regex
Same as [kluctl.io/ignore-diff-field](#kluctlioignore-diff-field) but specifying a regular expressions instead of a
JSON Path.

If more than one field needs to be specified, add `-xxx` to the annotation key, where `xxx` is an arbitrary number.
