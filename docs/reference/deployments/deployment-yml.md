---
title: "deployment.yaml"
linkTitle: "deployment.yaml"
weight: 1
description: >
    Structure of deployment.yaml.
---

The `deployment.yaml` file is the entrypoint for the deployment project. Included sub-deployments also provide a
`deployment.yaml` file with the same structure as the initial one.

An example `deployment.yaml` looks like this:
```yaml
sealedSecrets:
  outputPattern: "{{ cluster.name }}/{{ args.environment }}"

deployments:
- path: nginx
- path: my-app
- include: monitoring

commonLabels:
  my.prefix/target: "{{ target.name }}"
  my.prefix/deployment-project: my-deployment-project

args:
- name: environment
```

The following sub-chapters describe the available fields in the `deployment.yaml`

## sealedSecrets
`sealedSecrets` configures how sealed secrets are stored while sealing and located while rendering.
See [Sealed Secrets]({{< ref "docs/reference/sealed-secrets#outputpattern-and-location-of-stored-sealed-secrets" >}})
for details.

## deployments

`deployments` is a list of deployment items. Multiple deployment types are supported, which is documented further down.
Individual deployments are performed in parallel, unless a [barrier](#barriers) is encountered which causes kluctl to
wait for all previous deployments to finish.

### Kustomize deployments

Specifies a [kustomize](https://kustomize.io/) deployment.
Please see [Kustomize integration]({{< ref "./kustomize" >}}) for more details.

Example:
```yaml
deployments:
- path: path/to/deployment1
- path: path/to/deployment2
  waitReadiness: true
```

The `path` must point to a directory relative to the directory containing the `deployment.yaml`. Only directories
that are part of the kluctl project are allowed. The directory must contain a valid `kustomization.yaml`.

`waitReadiness` is optional and if set to `true` instructs kluctl to wait for readiness of each individual object
of the kustomize deployment. Readiness is defined in [readiness]({{< ref "./readiness" >}}).

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
    ref: my-branch
    subDir: some/sub/dir
```

The url specifies the Git url to be cloned and checked out. `ref` is optional and specifies the branch or tag to be used.
If `ref` is omitted, the default branch will be checked out. `subDir` is optional and specifies the sub directory inside
the git repository to include.

### Barriers
Causes kluctl to wait until all previous kustomize deployments have been applied. This is useful when
upcoming deployments need the current or previous deployments to be finished beforehand. Previous deployments also
include all sub-deployments from included deployments.

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

## deployments common properties
All entries in `deployments` can have the following common properties:

### vars (deployment item)
A list of variable sets to be loaded into the templating context, which is then available in all [deployment items](#deployments)
and [sub-deployments](#includes).

See [templating]({{< ref "docs/reference/templating#vars-from-deploymentyaml" >}}) for more details.

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

### tags (deployment item)
A list of tags the deployment should have. See [tags]({{< ref "./tags" >}}) for more details. For includes, this means that all
sub-deployments will get these tags applied to. If not specified, the default tags logic as described in [tags]({{< ref "./tags" >}})
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
See [Deploying with tag inclusion/exclusion]({{< ref "./tags#deploying-with-tag-inclusionexclusion" >}}) for details.

```yaml
deployments:
- path: kustomizeDeployment1
  alwaysDeploy: true
- path: kustomizeDeployment2
```

### skipDeleteIfTags
Forces exclusion of a deployment whenever inclusion/exclusion tags are specified via command line.
See [Deleting with tag inclusion/exclusion]({{< ref "./tags#deploying-with-tag-inclusionexclusion" >}}) for details.

```yaml
deployments:
- path: kustomizeDeployment1
  skipDeleteIfTags: true
- path: kustomizeDeployment2
```

## vars (deployment project)
A list of variable sets to be loaded into the templating context, which is then available in all [deployment items](#deployments)
and [sub-deployments](#includes).

See [templating]({{< ref "docs/reference/templating#vars-from-deploymentyaml" >}}) for more details.

## commonLabels
A dictionary of [labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) and values to be
added to all resources deployed by any of the kustomize deployments in this deployment project.

This feature is mainly meant to make it possible to identify all objects in a kubernetes cluster that were once deployed
through a specific deployment project.

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

Every resource deployed by the kustomize deployment `nginx` will now get the two provided labels attached. All included
sub-deployment projects (e.g. `sub-deployment1`) will also recursively inherit these labels and pass them to further
down.

In case an included sub-deployment project also contains `commonLabels`, both dictionaries of common labels are merged
inside the included sub-deployment project. In case of conflicts, the included common labels override the inherited.

The root deployment's `commonLabels` is also used to identify objects to be deleted when performing `kluctl delete`
or `kluctl prune` operations

Please note that these `commonLabels` are not related to `commonLabels` supported in `kustomization.yaml` files. It was
decided to not rely on this feature but instead attach labels manually to resources right before sending them to
kubernetes. This is due to an [implementation detail](https://github.com/kubernetes-sigs/kustomize/issues/1009) in
kustomize which causes `commonLabels` to also be applied to label selectors, which makes otherwise editable resources
read-only when it comes to `commonLabels`.

## overrideNamespace
A string that is used as the default namespace for all kustomize deployments which don't have a `namespace` set in their
`kustomization.yaml`.

## tags (deployment project)
A list of common tags which are applied to all kustomize deployments and sub-deployment includes.

See [tags]({{< ref "./tags" >}}) for more details.

## args
A list of arguments that can or must be passed to most kluctl operations. Each of these arguments is then available
in templating via the global `args` object. Only the root `deployment.yaml` can contain such argument definitions.

An example looks like this:
```yaml
deployments:
  - path: nginx

args:
  - name: environment
  - name: enable_debug
    default: false
  - name: complex_arg
    default:
      my:
        nested1: arg1
        nested2: arg2
```

These arguments can then be used in templating, e.g. by using `{{ args.environment }}`.

When calling kluctl, most of the commands will then require you to specify at least `-a environment=xxx` and optionally
`-a enable_debug=true`

The following sub chapters describe the fields for argument entries.

### name
The name of the argument.

### default
If specified, the argument becomes optional and will use the given value as default when not specified.

The default value can be an arbitrary yaml value, meaning that it can also be a nested dictionary. In that case, passing
args in nested form will only set the nested value. With the above example of `complex_arg`, running:

```
kluctl deploy -t my-target -a my.nested1=override`
```

will only modify the value below `my.nested1` and keep the value of `my.nested2`.

## ignoreForDiff

A list of objects and fields to ignore while performing diffs. Consider the following example:

```yaml
deployments:
  - ...

ignoreForDiff:
  - group: apps
    kind: Deployment
    namespace: my-namespace
    name: my-deployment
    fieldPath: spec.replicas
```

This will remove the `spec.replicas` field from every resource that matches the object.
`group`, `kind`, `namespace` and `name` can be omitted, which results in all objects matching. `fieldPath` must be a
valid [JSON Path](https://goessner.net/articles/JsonPath/). `fieldPath` may also be a list of JSON paths.

The JSON Path implementation used in kluctl has extended support for wildcards in field
names, allowing you to also specify paths like `metadata.labels.my-prefix-*`.

As an alternative, [annotations]({{< ref "./annotations/all-resources#control-diff-behavior" >}}) can be used to control
diff behavior of individual resources.
