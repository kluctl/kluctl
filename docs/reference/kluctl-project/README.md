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
```

## Allowed fields

Please check the following sub-sections of this section to see which fields are allowed at the root level of `.kluctl.yaml`.

1. [targets](./targets)

## Using Kluctl without .kluctl.yaml

It's possible to use Kluctl without any `.kluctl.yaml`. In that case, all commands must be used without specifying the
target.
