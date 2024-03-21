<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Deployments"
linkTitle: "Deployments"
weight: 1
description: >
    Structure of deployment.yaml.
---
-->

# Deployments

The `deployment.yaml` file is the entrypoint for the deployment project. Included sub-deployments also provide a
`deployment.yaml` file with the same structure as the initial one.

An example `deployment.yaml` looks like this:
```yaml
deployments:
- path: nginx
- path: my-app
- include: monitoring
- git:
    url: git@github.com/example/example.git
- oci:
    url: oci://ghcr.io/kluctl/kluctl-examples/simple

commonLabels:
  my.prefix/target: "{{ target.name }}"
  my.prefix/deployment-project: my-deployment-project
```

The following sub-chapters describe the available fields in the `deployment.yaml`

## deployments

`deployments` is a list of deployment items. Multiple deployment types are supported, which is documented further down.
Individual deployments are performed in parallel, unless a [barrier](#barriers) is encountered which causes kluctl to
wait for all previous deployments to finish.

Deployments can also be conditional by using the [when](#when) field.

### Simple deployments

Simple deployments are specified via `path` and are expected to be directories with Kubernetes manifests inside.
Kluctl will internally generate a kustomization.yaml from these manifests and treat the deployment item the same way
as it would treat a [Kustomize deployment](#kustomize-deployments).

Example:
```yaml
deployments:
- path: path/to/manifests
```

### Kustomize deployments

When the deployment item directory specified via `path` contains a `kustomization.yaml`, Kluctl will use this file
instead of generating one.

Please see [Kustomize integration](./kustomize.md) for more details.

Example:
```yaml
deployments:
- path: path/to/deployment1
- path: path/to/deployment2
```

The `path` must point to a directory relative to the directory containing the `deployment.yaml`. Only directories
that are part of the kluctl project are allowed. The directory must contain a valid `kustomization.yaml`.

### Includes

Specifies a sub-deployment project to be included. The included sub-deployment project will inherit many properties
of the parent project, e.g. tags, commonLabels and so on.

Example:
```yaml
deployments:
- include: path/to/sub-deployment
```

The `path` must point to a directory relative to the directory containing the `deployment.yaml`. Only directories
that are part of the kluctl project are allowed. The directory must contain a valid `deployment.yaml`.

### Git includes

Specifies an external git project to be included. The project is included the same way with regular includes, except
that the included project can not use/load templates from the parent project. An included project might also include
further git projects.

If the included project is a [Kluctl Library Project](../kluctl-libraries/README.md), current variables are NOT passed
automatically into the included project. Only when [passVars](#passvars) is set to true, all current variables are passed.
For library projects, [args](#args) is the preferred way to pass configuration.

Simple example:
```yaml
deployments:
- git: git@github.com/example/example.git
```

This will clone the git repository at `git@github.com/example/example.git`, checkout the default branch and include it
into the current project.

Advanced Example:
```yaml
deployments:
- git:
    url: git@github.com/example/example.git
    ref:
      branch: my-branch
    subDir: some/sub/dir
```

The url specifies the Git url to be cloned and checked out.

`ref` is optional and specifies the branch or tag to be used. To specify a branch, set the sub-field `branch` as seen
in the above example. To pass a tag, set the `tag` field instead. To pass a commit, set the `commit` field instead.

If `ref` is omitted, the default branch will be checked out.

`subDir` is optional and specifies the sub directory inside the git repository to include.

### OCI includes

Specifies an OCI based artifact to include. The artifact must be pushed to your OCI repository via the
[`kluctl oci push`](../commands/oci-push.md) command. The artifact is extracted and then included the same way a
[git include](#git-includes) is included.

If the included project is a [Kluctl Library Project](../kluctl-libraries/README.md), current variables are NOT passed
automatically into the included project. Only when [passVars](#passvars) is set to true, all current variables are passed.
For library projects, [args](#args) is the preferred way to pass configuration.

Simple example:
```yaml
deployments:
- oci:
    url: oci://ghcr.io/kluctl/kluctl-examples/simple
```

The `url` specifies the OCI repository url. It must use the `oci://` scheme. It is not allowed to add tags or digests to
the url. Instead, use the dedicated `ref` field:

```yaml
deployments:
- oci:
    url: oci://ghcr.io/kluctl/kluctl-examples/simple
    ref:
      tag: latest
```

For digests, use:

```yaml
deployments:
- oci:
    url: oci://ghcr.io/kluctl/kluctl-examples/simple
    ref:
      digest: sha256:9ac3ba762c373ebccecb9dd3ac1d8ca091e4bd4a101701ce99e6058c0c74eedc
```

Subdirectories of the pushed artifact can be specified via `subDir`:

```yaml
deployments:
- oci:
    url: oci://ghcr.io/kluctl/kluctl-examples/simple
    subDir: my-subdir
```

See [OCI support](./oci.md) for more details, especially in regard to authentication for private registries.

### Barriers
Causes kluctl to wait until all previous kustomize deployments have been applied. This is useful when
upcoming deployments need the current or previous deployments to be finished beforehand. Previous deployments also
include all sub-deployments from included deployments.

Please note that barriers do not wait for readiness of individual resources. This means that it will not wait for
readiness of services, deployments, daemon sets, and so on. To actually wait for readiness, use `waitReadiness: true` or
`waitReadinessObjects`.

Example:
```yaml
deployments:
- path: kustomizeDeployment1
- path: kustomizeDeployment2
- include: subDeployment1
- barrier: true
# At this point, it's ensured that kustomizeDeployment1, kustomizeDeployment2 and all sub-deployments from
# subDeployment1 are fully deployed.
- path: kustomizeDeployment3
```

To create a barrier with a custom message, include the message parameter when creating the barrier. The message parameter accepts a string value that represents the custom message.

Example:
```yaml
deployments:
- path: kustomizeDeployment1
- path: kustomizeDeployment2
- include: subDeployment1
- barrier: true
  message: "Waiting for subDeployment1 to be finished"
# At this point, it's ensured that kustomizeDeployment1, kustomizeDeployment2 and all sub-deployments from
# subDeployment1 are fully applied.
- path: kustomizeDeployment3
```
If no custom message is provided, the barrier will be created without a specific message, and the default behavior will be applied.

When viewing the `kluctl deploy` status, the custom message, if provided, will be displayed along with default barrier information.

### waitReadiness
`waitReadiness` can be set on all deployment items. If set to `true`, Kluctl will wait for readiness of each individual object
of the current deployment item. Readiness is defined in [readiness](./readiness.md).

Please note that Kluctl will not wait for readiness of previous deployment items.

This can also be combined with [barriers](#barriers), which will instruct Kluctl to stop processing the next deployment
items until everything before the barrier is applied and the current deployment item's objects are all ready.

Examples:
```yaml
deployments:
- path: kustomizeDeployment1
  waitReadiness: true
- path: kustomizeDeployment2
  # this will wait for kustomizeDeployment1 to be applied+ready and kustomizeDeployment2 to be applied
  # kustomizeDeployment2 is not guaranteed to be ready at this point, but might be due to the parallel nature of Kluctl
- barrier: true
- path: kustomizeDeployment3
```

### waitReadinessObjects
This is comparable to `waitReadiness`, but instead of waiting for all objects of the current deployment item, it allows
to explicitly specify objects which are not necessarily part of the current (or any) deployment item.

This is for example useful if you used an external Helm Chart and want to wait for readiness of some individual objects,
e.g. CRDs that are being deployment by some in-cluster operator instead of the Helm chart itself.

Examples:
```yaml
deployments:
# The cilium Helm chart does not deploy CRDs anymore. Instead, the cilium-operator does this on startup. This means,
# we can't apply CiliumNetworkPolicies before the CRDs get applied by the operator.
- path: cilium
- barrier: true
  waitReadinessObjects:
  - kind: Deployment
    name: cilium-operator
    namespace: kube-system
  - kind: CustomResourceDefinition
    name: ciliumnetworkpolicies.cilium.io
# This deployment can now safely use the CRDs applied by the operator
- path: kustomizeDeployment1
```

### deleteObjects
Causes kluctl to delete matching objects, specified by a list of group/kind/name/namespace dictionaries.
The order/parallelization of deletion is identical to the order and parallelization of normal deployment items,
meaning that it happens in parallel by default until a barrier is encountered.

Example:
```yaml
deployments:
  - deleteObjects:
      - group: apps
        kind: DaemonSet
        namespace: kube-system
        name: kube-proxy
  - barrier: true
  - path: my-cni
```

The above example shows how to delete the kube-proxy DaemonSet before installing a CNI (e.g. Cilium in
proxy-replacement mode).

## deployments common properties
All entries in `deployments` can have the following common properties:

### vars (deployment item)
A list of variable sets to be loaded into the templating context, which is then available in all [deployment items](#deployments)
and [sub-deployments](#includes).

See [templating](../templating/variable-sources.md) for more details.

Example:
```yaml
deployments:
- path: kustomizeDeployment1
  vars:
    - file: vars1.yaml
    - values:
        var1: value1
- path: kustomizeDeployment2
# all sub-deployments of this include will have the given variables available in their Jinj2 context.
- include: subDeployment1
  vars:
    - file: vars2.yaml
```

### passVars
Can only be used on [include](#includes), [git include](#git-includes) and [oci include](#oci-includes). If set to `true`,
all variables will be passed down to the included project even if the project is an explicitly marked
[Kluctl Library Project](../kluctl-libraries/README.md).

If the included project is not a library project, variables are always fully passed into the included deployment. 

### args
Can only be used on [include](#includes), [git include](#git-includes) and [oci include](#oci-includes). Passes the given
arguments into [Kluctl Library Projects](../kluctl-libraries/README.md).

### when
Each deployment item can be conditional with the help of the `when` field. It must be set to a
[Jinja2 based expression](https://jinja.palletsprojects.com/en/3.1.x/templates/#expressions)
that evaluates to a boolean.

Example:
```yaml
deployments:
- path: item1
- path: item2
  when: my.var == "my-value"
```

### tags (deployment item)
A list of tags the deployment should have. See [tags](./tags.md) for more details. For includes, this means that all
sub-deployments will get these tags applied to. If not specified, the default tags logic as described in [tags](./tags.md)
is applied.

Example:

```yaml
deployments:
- path: kustomizeDeployment1
  tags:
    - tag1
    - tag2
- path: kustomizeDeployment2
  tags:
    - tag3
# all sub-deployments of this include will get tag4 applied
- include: subDeployment1
  tags:
    - tag4
```

### alwaysDeploy
Forces a deployment to be included everytime, ignoring inclusion/exclusion sets from the command line.
See [Deploying with tag inclusion/exclusion](./tags.md#deploying-with-tag-inclusionexclusion) for details.

```yaml
deployments:
- path: kustomizeDeployment1
  alwaysDeploy: true
- path: kustomizeDeployment2
```

Please note that `alwaysDeploy` will also cause [kluctl render](../commands/render.md) to always render the resources.

### skipDeleteIfTags
Forces exclusion of a deployment whenever inclusion/exclusion tags are specified via command line.
See [Deleting with tag inclusion/exclusion](./tags.md#deploying-with-tag-inclusionexclusion) for details.

```yaml
deployments:
- path: kustomizeDeployment1
  skipDeleteIfTags: true
- path: kustomizeDeployment2
```

### onlyRender
Causes a path to be rendered only but not treated as a deployment item. This can be useful if you for example want to
use Kustomize components which you'd refer from other deployment items.

```yaml
deployments:
- path: component
  onlyRender: true
- path: kustomizeDeployment2
```

## vars (deployment project)
A list of variable sets to be loaded into the templating context, which is then available in all [deployment items](#deployments)
and [sub-deployments](#includes).

See [templating](../templating/variable-sources.md) for more details.

## commonLabels
A dictionary of [labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) and values to be
added to all resources deployed by any of the deployment items in this deployment project.

Consider the following example `deployment.yaml`:
```yaml
deployments:
  - path: nginx
  - include: sub-deployment1

commonLabels:
  my.prefix/target: {{ target.name }}
  my.prefix/deployment-name: my-deployment-project-name
  my.prefix/label-1: value-1
  my.prefix/label-2: value-2
```

Every resource deployed by the kustomize deployment `nginx` will now get the four provided labels attached. All included
sub-deployment projects (e.g. `sub-deployment1`) will also recursively inherit these labels and pass them further
down.

In case an included sub-deployment project also contains `commonLabels`, both dictionaries of commonLabels are merged
inside the included sub-deployment project. In case of conflicts, the included common labels override the inherited.

Please note that these `commonLabels` are not related to `commonLabels` supported in `kustomization.yaml` files. It was
decided to not rely on this feature but instead attach labels manually to resources right before sending them to
kubernetes. This is due to an [implementation detail](https://github.com/kubernetes-sigs/kustomize/issues/1009) in
kustomize which causes `commonLabels` to also be applied to label selectors, which makes otherwise editable resources
read-only when it comes to `commonLabels`.

## commonAnnotations
A dictionary of [annotations](https://kubernetes.io/docs/concepts/overview/working-with-objects/annotations/) and values to be
added to all resources deployed by any of the deployment items in this deployment project.

`commonAnnotations` are handled the same as [commonLabels](#commonlabels) in regard to inheriting, merging and overriding.

## overrideNamespace
A string that is used as the default namespace for all kustomize deployments which don't have a `namespace` set in their
`kustomization.yaml`.

## tags (deployment project)
A list of common tags which are applied to all kustomize deployments and sub-deployment includes.

See [tags](./tags.md) for more details.

## ignoreForDiff

A list of rules used to determine which differences should be ignored in diff outputs.

As an alternative, [annotations](./annotations/all-resources.md#control-diff-behavior) can be used to control
diff behavior of individual resources.

Consider the following example:

```yaml
deployments:
  - ...

ignoreForDiff:
  - kind: Deployment
    name: my-deployment
    fieldPath: spec.replicas
```

This will ignore differences for the `spec.replicas` field in the `Deployment` with the name `my-deployment`.

Using regex expressions instead of JSON Pathes is also supported:

```yaml
deployments:
  - ...

ignoreForDiff:
  - kind: Deployment
    name: my-deployment
    fieldPathRegex: metadata.labels.my-label-.*
```

The following properties are supported in `ignoreForDiff` items.

### fieldPath
If specified, must be a valid [JSON Path](https://goessner.net/articles/JsonPath/). Kluctl will ignore differences for
all matching fields of all matching objects (see the other properties).

Either `fieldPath` or `fieldPathRegex` must be provided.

### fieldPathRegex
If specified, must be a valid regex. Kluctl will ignore differences for all matching fields of all matching objects
(see the other properties).

Either `fieldPath` or `fieldPathRegex` must be provided.

### group
This property is optional. If specified, only objects with a matching api group will be considered. Please note that this
field should NOT include the version of the api group.

### kind
This property is optional. If specified, only objects with a matching `kind` will be considered.

### namespace
This property is optional. If specified, only objects with a matching `namespace` will be considered.

### name
This property is optional. If specified, only objects with a matching `name` will be considered.

## conflictResolution

A list of rules used to determine how to handle conflict resolution.

As an alternative, [annotations](./annotations/all-resources.md#control-deploy-behavior) can be used to control
conflict resolution of individual resources.

Consider the following example:

```yaml
deployments:
  - ...

conflictResolution:
  - kind: ValidatingWebhookConfiguration
    fieldPath: webhooks.*.*
    action: ignore
```

This will cause Kluctl to ignore conflicts on all matching fields of all `ValidatingWebhookConfiguration` objects.

Using regex expressions instead of JSON Pathes is also supported:

```yaml
deployments:
  - ...

conflictResolution:
  - kind: ValidatingWebhookConfiguration
    fieldPathRegex: webhooks\..
    action: ignore
```

The following properties are supported in `conflictResolution` items.

### fieldPath
If specified, must be a valid [JSON Path](https://goessner.net/articles/JsonPath/). Kluctl will ignore conflicts for
all matching fields of all matching objects (see the other properties).

Either `fieldPath` or `fieldPathRegex` must be provided.

### fieldPathRegex
If specified, must be a valid regex. Kluctl will ignore conflicts for all matching fields of all matching objects
(see the other properties).

Either `fieldPath` or `fieldPathRegex` must be provided.

### action
This field is required and must be either `ignore` or `force-apply`. 

### group
This property is optional. If specified, only objects with a matching api group will be considered. Please note that this
field should NOT include the version of the api group.

### kind
This property is optional. If specified, only objects with a matching `kind` will be considered.

### namespace
This property is optional. If specified, only objects with a matching `namespace` will be considered.

### name
This property is optional. If specified, only objects with a matching `name` will be considered.
