<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "targets"
linkTitle: "targets"
weight: 4
description: >
  Required, defines targets for this kluctl project.
---
-->

# targets

Specifies a list of targets for which commands can be invoked. A target puts together environment/target specific
configuration and the target cluster. Multiple targets can exist which target the same cluster but with differing
configuration (via `args`).

Each value found in the target definition is rendered with a simple Jinja2 context that only contains the target and args.
The rendering process is retried 10 times until it finally succeeds, allowing you to reference
the target itself in complex ways.

Target entries have the following form:
```yaml
targets:
...
  - name: <target_name>
    context: <context_name>
    args:
      arg1: <value1>
      arg2: <value2>
      ...
    images:
      - image: my-image
        resultImage: my-image:1.2.3
    aws:
      profile: my-local-aws-profile
      serviceAccount:
        name: service-account-name
        namespace: service-account-namespace
    discriminator: "my-project-{{ target.name }}"
...
```

The following fields are allowed per target:

## name
This field specifies the name of the target. The name must be unique. It is referred in all commands via the
[-t](../../commands/common-arguments.md) option.

## context
This field specifies the kubectl context of the target cluster. The context must exist in the currently active kubeconfig.
If this field is omitted, Kluctl will always use the currently active context.

## args
This fields specifies a map of arguments to be passed to the deployment project when it is rendered. Allowed argument names
are configured via [deployment args](../../deployments/deployment-yml.md#args).

## images
This field specifies a list of fixed images to be used by [`images.get_image(...)`](../../deployments/images.md#imagesget_image).
The format is identical to the [fixed images file](../../deployments/images.md#command-line-argument---fixed-images-file).

## aws
This field specifies target specific AWS configuration, which overrides what was optionally specified via the
[global AWS configuration](../README.md#aws).

## discriminator

Specifies a discriminator which is used to uniquely identify all deployed objects on the cluster. It is added to all
objects as the value of the `kluctl.io/discriminator` label. This label is then later used to identify all objects
belonging to the deployment project and target, so that Kluctl can determine which objects got orphaned and need to
be pruned. The discriminator is also used to identify all objects that need to be deleted when
[kluctl delete](../../commands/delete.md) is called.

If no discriminator is set for a target, [kluctl prune](../../commands/prune.md) and
[kluctl delete](../../commands/delete.md) are not supported.

The discriminator can be a [template](../../templating/README.md) which is rendered at project loading time. While
rendering, only the `target` and `args` are available as global variables in the templating context.

The rendered discriminator should be unique on the target cluster to avoid mis-identification of objects from other
deployments or targets. It's good practice to prefix the discriminator with a project name and at least use the target
name to make it unique. Example discriminator to achieve this: `my-project-name-{{ target.name }}`.

If a target is meant to be deployed multiple times, e.g. by using external [arguments](../README.md#args), the external
arguments should be taken into account as well. Example: `my-project-name-{{ target.name }}-{{ args.environment_name }}`.

A [default discriminator](../../kluctl-project/README.md#discriminator) can also be specified which is used whenever
a target has no discriminator configured.
