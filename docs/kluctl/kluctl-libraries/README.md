<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Kluctl Library Projects"
linkTitle: "Kluctl Library Projects"
weight: 20
description: >
    Kluctl library project configuration, found in the .kluctl-library.yaml file.
---
-->

# Kluctl Library Projects

A library project is a Kluctl deployment that is meant to be included by other projects. It can be provided with
configuration either via [args](#args) or via [vars in the include](../deployments/deployment-yml.md#vars-deployment-item).

Kluctl [deployment projects](../deployments/README.md) can include these library projects via
[local include](../deployments/deployment-yml.md#includes), [Git include](../deployments/deployment-yml.md#git-includes)
or [Oci includes](../deployments/deployment-yml.md#oci-includes). 
artifacts.

The `.kluctl-library.yaml` marks a deployment project as a library project and provides some configuration.

## Example

Consider the following root `deployment.yaml` inside your root project:

```yaml
deployments:
  - git:
      url: git@github.com/example/example-library.git
    args:
      arg1: value1
```

And the following `.kluctl-library.yaml` inside the included `example-library` git project:

```yaml
args:
  - name: arg1
  - name: arg2
    default: value2
```

This will include the given git repository and make `args.arg1` and `args.arg2` available via [templating](../templating/README.md).

## Allowed fields

### args

A list of arguments that can or must be passed when including the library project. Each of these arguments is then available
in templating via the global `args` object.

An example looks like this:
```yaml
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

The meaning and function of these arguements is identical to the [args in .kluctl.yaml](../kluctl-project/README.md#args).

## Using Kluctl Libraries without .kluctl-library.yaml

Includes can also be done on projects that do not have a `.kluctl-library.yaml` configuration. In that case, all
currently available variables are passed into the include project, including the `args` from the root deployment project.
