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

The `.kluctl.yaml` is the central configuration and entry point for your deployments. It defines where the actual
[deployment project](../deployments) is located,
where [sealed secrets](../sealed-secrets.md) and unencrypted secrets are localed and which targets are available to
invoke [commands](../commands) on.

## Example

An example .kluctl.yaml looks like this:

```yaml
targets:
  # test cluster, dev env
  - name: dev
    context: test.example.com
    args:
      environment_name: dev
    sealingConfig:
      secretSets:
        - non-prod
  # test cluster, test env
  - name: test
    context: test.example.com
    args:
      environment_name: test
    sealingConfig:
      secretSets:
        - non-prod
  # prod cluster, prod env
  - name: prod
    context: prod.example.com
    args:
      environment_name: prod
    sealingConfig:
      secretSets:
        - prod

# This is only required if you actually need sealed secrets
secretsConfig:
  secretSets:
    - name: prod
      vars:
        # This file should not be part of version control!
        - file: .secrets-prod.yaml
    - name: non-prod
      vars:
        # This file should not be part of version control!
        - file: .secrets-non-prod.yaml
```

## Allowed fields

Please check the following sub-sections of this section to see which fields are allowed at the root level of `.kluctl.yaml`.

1. [targets](./targets)
2. [secretsConfig](./secrets-config)
