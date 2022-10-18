---
title: "Dynamic Targets"
linkTitle: "Dynamic Targets"
weight: 1
description: >
    Dynamically defined targets.
---

Targets can also be "dynamic", meaning that additional configuration can be sourced from another git repository.
This can be based on a single target repository and branch, or on a target repository and branch/ref pattern, resulting
in multiple dynamic targets being created from one target definition.

Please note that a single entry in `target` might end up with multiple dynamic targets, meaning that the name must be
made unique between these dynamic targets. This can be achieved by using templating in the `name` field. As an example,
`{{ target.targetConfig.ref }}` can be used to set the target name to the branch name of the dynamic target.

Dynamic targets have the following form:
```yaml
targets:
...
  - name: <dynamic_target_name>
    context: <cluster_name>
    args: ...
      arg1: <value1>
      arg2: <value2>
      ...
    targetConfig:
      project:
        url: <git-url>
      ref: <ref-name>
      refPattern: <regex-pattern>
      file: <config-file>
    sealingConfig:
      dynamicSealing: <false_or_true>
      secretSets:
        - <name_of_secrets_set>
...
```

All fields known from normal targets are allowed. In addition, the targetConfig with following fields is available.

## targetConfig

The presence of this field causes the target to become a dynamic target. 
It specifies where to look for dynamic targets and their addional configuration. It has the following form:

```yaml
...
targets:
...
- name: <dynamic_target_name>
  ...
  targetConfig:
    project:
      url: <git-url>
    ref: <ref-name>
    refPattern: <regex-pattern>
...
```

### project.url
This field specifies the git clone url of the target configuration project.

### ref
This field specifies the branch or tag to use. If this field is specified, using `refPattern` is forbidden.
This will result in one single dynamic target.

### refPattern
This field specifies a regex pattern to use when looking for candidate branches and tags. If this is specified,
using `ref` is forbidden. This will result in multiple dynamic targets. Each dynamic target will have `ref` set to
the actual branch name it belong to. This allows using of `{{ target.targetConfig.ref }}` in all other target fields.

### file
This field specifies the config file name to read externalized target config from.

## Format of the target config
The target config file referenced in `targetConfig` must be of the following format:

```yaml
args:
  arg1: value1
  arg2: value2
images:
  - image: registry.gitlab.com/my-group/my-project
    resultImage: registry.gitlab.com/my-group/my-project:1.1.0
```

### args
An optional map of arguments, in the same format as in the normal [target args]({{< ref "docs/reference/kluctl-project/targets/#args" >}}).

The arguments specified here have higher priority.

### images
An optional list of fixed images, in the same format as in the normal [target images]({{< ref "docs/reference/kluctl-project/targets/#images" >}})

## Simple dynamic targets

A simplified form of dynamic targets is to store target config inside the same directory/project as the `.kluctl.yaml`.
This can be done by omitting `project`, `ref` and `refPattern` from `targetConfig` and only specify `file`.

## A note on sealing

When sealing dynamic targets, it is very likely that it is not known yet which dynamic targets will actually exist in
the future. This requires some special care when sealing secrets for these targets. Sealed secrets are usually namespace
scoped, which might need to be changed to cluster-wide scoping so that the same sealed secret can be deployed into
multiple targets (assuming you deploy to different namespaces for each target). When you do this, watch out to not
compromise security, e.g. by sealing production level secrets with a cluster-wide scope!

It is also very likely required to set `target.sealingConfig.dynamicSealing` to `false`, so that sealing is only performed
once and not for all dynamic targets.