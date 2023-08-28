<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "SOPS Integration"
linkTitle: "SOPS Integration"
weight: 5
description: >
    How SOPS is integrated into Kluctl
---
-->

# SOPS Integration

Kluctl integrates natively with [SOPS](https://github.com/mozilla/sops). Kluctl is able to decrypt all resources
referenced by [Kustomize](./kustomize.md) deployment items (including [simple deployments](./deployment-yml.md#simple-deployments)).
In addition, Kluctl will also decrypt all variable sources of the types [file](../templating/variable-sources.md#file)
and [git](../templating/variable-sources.md#git).

Kluctl assumes that you have setup sops as usual so that it knows how to decrypt these files.

## Only encrypting Secrets's data

To only encrypt the `data` and `stringData` fields of Kubernetes secrets, use a `.sops.yaml` configuration file that
`encrypted_regex` to filter encrypted fields:

```
creation_rules:
  - path_regex: .*.yaml
    encrypted_regex: ^(data|stringData)$
```

## Combining templating and SOPS

As an alternative, you can split secret values and the resulting Kubernetes resources into two different places and then
use templating to use the secret values wherever needed. Example:

Write the following content into `secrets/my-secrets.yaml`:

```yaml
secrets:
  mySecret: secret-value
```

And encrypt it with SOPS:

```shell
$ sops -e -i secrets/my-secrets.yaml
```

Add this [variables source](../templating/variable-sources.md) to one of your [deployments](./deployment-yml.md):

```yaml
vars:
  - file: secrets/my-secrets.yaml

deployments:
- ...
```

Then, in one of your deployment items define the following `Secret`:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
  namespace: default
stringData:
  secret: "{{ secrets.mySecret }}"
```
