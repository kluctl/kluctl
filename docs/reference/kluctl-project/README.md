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
targets:
  # test cluster, dev env
  - name: dev
    context: test.example.com
    args:
      environment_name: dev
  # test cluster, test env
  - name: test
    context: test.example.com
    args:
      environment_name: test
  # prod cluster, prod env
  - name: prod
    context: prod.example.com
    args:
      environment_name: prod

args:
  - name: environment
```

## Allowed fields

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

## Using Kluctl without .kluctl.yaml

It's possible to use Kluctl without any `.kluctl.yaml`. In that case, all commands must be used without specifying the
target.
