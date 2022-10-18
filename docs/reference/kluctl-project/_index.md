---
title: "Kluctl project (.kluctl.yaml)"
linkTitle: ".kluctl.yaml"
weight: 1
description: >
    Kluctl project configuration, found in the .kluctl.yaml file.
---

The `.kluctl.yaml` is the central configuration and entry point for your deployments. It defines where the actual
[deployment project]({{< ref "docs/reference/deployments" >}}) is located,
where [sealed secrets]({{< ref "docs/reference/sealed-secrets" >}}) and unencrypted secrets are localed and which targets are available to
invoke [commands]({{< ref "docs/reference/commands" >}}) on.

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

Please check the sub-sections of this section to see which fields are allowed at the root level of `.kluctl.yaml`.
