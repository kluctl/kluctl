<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Kluctl project"
linkTitle: ".kluctl.yaml"
weight: 1
description: >
    Kluctl project configuration, found in the .kluctl.yaml file.
---
-->

# Kluctl project

The `.kluctl.yaml` is the central configuration and entry point for your deployments. It defines which targets are
available to invoke [commands](../commands) on.

## Example

An example .kluctl.yaml looks like this:

```yaml
discriminator: "my-project-{{ target.name }}"

targets:
  # test cluster, dev env
  - name: dev
    context: dev.example.com
    args:
      environment: dev
  # test cluster, test env
  - name: test
    context: test.example.com
    args:
      environment: test
  # prod cluster, prod env
  - name: prod
    context: prod.example.com
    args:
      environment: prod

args:
  - name: environment
```

## Allowed fields

### discriminator

Specifies a default discriminator template to be used for targets that don't have
their own discriminator specified.

See [target discriminator](./targets/#discriminator) for details.

### targets

Please check the [targets](./targets) sub-section for details.

### args

A list of arguments that can or must be passed to most kluctl operations. Each of these arguments is then available
in templating via the global `args` object.

An example looks like this:
```yaml
targets:
...

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

#### name
The name of the argument.

#### default
If specified, the argument becomes optional and will use the given value as default when not specified.

The default value can be an arbitrary yaml value, meaning that it can also be a nested dictionary. In that case, passing
args in nested form will only set the nested value. With the above example of `complex_arg`, running:

```
kluctl deploy -t my-target -a my.nested1=override`
```

will only modify the value below `my.nested1` and keep the value of `my.nested2`.

### aws
If specified, configures the default AWS configuration to use for
[awsSecretsManager](../templating/variable-sources.md#awssecretsmanager) vars sources and KMS based
[SOPS descryption](../deployments/sops.md).

Example:

```yaml
aws:
  profile: my-local-aws-profile
  serviceAccount:
    name: service-account-name
    namespace: service-account-namespace
```

#### profile
If specified, Kluctl will use this [AWS config profile](https://docs.aws.amazon.com/cli/latest/userguide/cli-configure-files.html#cli-configure-files-using-profiles)
when found locally. If it is not found in your local AWS config, Kluctl will not try to use the specified profile.

#### serviceAccount
Optionally specifies the name and namespace of a service account to use for [IRSA based authentication](https://docs.aws.amazon.com/eks/latest/userguide/iam-roles-for-service-accounts.html).
The specified service accounts needs to have the `eks.amazonaws.com/role-arn` annotation set to an existing IAM role
with a proper trust policy that allows this service account to assume that role. Please read the AWS documentation
for details.

The service account is only used when [profile](#profile) was not specified or when it is not present locally.
If a service account is specified and accessible (you need proper RBAC access), Kluctl will not try to perform default
AWS config loading.

## Using Kluctl without .kluctl.yaml

It's possible to use Kluctl without any `.kluctl.yaml`. In that case, all commands must be used without specifying the
target.
