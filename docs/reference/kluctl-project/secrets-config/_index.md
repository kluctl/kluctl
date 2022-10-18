---
title: "secretsConfig"
linkTitle: "secretsConfig"
weight: 4
description: >
  Optional, defines where to load secrets from.
---

This configures how secrets are retrieved while sealing. It is basically a list of named secret sets which can be
referenced from targets.

It has the following form:
```yaml
...
secretsConfig:
  secretSets:
    - name: <name>
      vars:
        - ...
  sealedSecrets: ...
...
```

## secretSets

Each `secretSets` entry has the following fields.

### name
This field specifies the name of the secret set. The name can be used in targets to refer to this secret set.

### vars
A list of variables sources. Check the documentation of
[variables sources]({{< ref "docs/reference/templating/variable-sources" >}}) for details.

Each variables source must have a root dictionary with the name `secrets` and all the actual secret values
below that dictionary. Every other root key will be ignored.

Example variables file:

```yaml
secrets:
  secret: value1
  nested:
    secret: value2
    list:
      - a
      - b
...
```

## sealedSecrets
This field specifies the configuration for sealing. It has the following form:

```yaml
...
secretsConfig:
  secretSets: ...
  sealedSecrets:
    bootstrap: true
    namespace: kube-system
    controllerName: sealed-secrets-controller
...
```

### bootstrap
Controls whether kluctl should bootstrap the initial private key in case the controller is not yet installed on
the target cluster. Defaults to `true`.

### namespace
Specifies the namespace where the sealed-secrets controller is installed. Defaults to "kube-system".

### controllerName
Specifies the name of the sealed-secrets controller. Defaults to "sealed-secrets-controller".
