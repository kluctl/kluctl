---
title: "Sealed Secrets"
linkTitle: "Sealed Secrets"
weight: 2
description: >
    Sealed Secrets integration
---

kluctl has an integration for [sealed secrets](https://github.com/bitnami-labs/sealed-secrets), allowing you to
securely store secrets for multiple target clusters and/or environments inside version control.

The integration consists of two parts:
1. Sealing of secrets
1. Automatically choosing and deploying the correct sealed secrets for a target

## Requirements

The Sealed Secrets integration relies on the [sealed-secrets operator](https://github.com/bitnami-labs/sealed-secrets)
being installed. Installing the operator is the responsibility of you (or whoever is managing/operating the cluster).

Kluctl can however perform sealing of secrets without an existing sealed-secrets operator installation. This is solved
by automatically pre-provisioning a key onto the cluster that is compatible with the operator or by providing the
public certificate via `certFile` in the targets [sealingConfig]({{< ref "docs/reference/kluctl-project/targets#certfile" >}}).

## Sealing of .sealme files

Sealing is done via the [seal command]({{< ref "docs/reference/commands/seal" >}}). It must be done before the actual
deployment is performed.

The `seal` command recursively searches for files that end with `.sealme`, renders them with the
[templating engine]({{< ref "docs/reference/templating" >}}) engine. The rendered secret resource is then
converted/encrypted into a sealed secret.

The `.sealme` files itself have to be [Kubernetes Secrets](https://kubernetes.io/docs/concepts/configuration/secret/),
but without any actual secret data inside. The secret data is referenced via templating variables and is expected to be
provided only at the time of sealing. This means, that the sensitive secret data must only be in clear text while sealing.
Afterwards the sealed secrets can be added to version control.

Example file (the name could be for example `db-secrets.yaml.sealme`):
```yaml
kind: Secret
apiVersion: v1
metadata:
  name: db-secrets
  namespace: {{ my.namespace.variable }}
stringData:
  DB_URL: {{ secrets.database.url }}
  DB_USERNAME: {{ secrets.database.username }}
  DB_PASSWORD: {{ secrets.database.password }}
```

While sealing, the full templating context (same as in [templating]({{< ref "docs/reference/templating" >}})) is available.
Additionally, the global `secrets` object/variable is available which contains the sensitive secrets.

## Secret Sources

Secrets are only loaded while sealing. Available secret sets and sources are configured via
[.kluctl.yaml]({{< ref "docs/reference/kluctl-project/secrets-config" >}}). The secrets used per target are configured via the
[secrets config]({{< ref "docs/reference/kluctl-project/targets#secretsets" >}}) of the targets.

## Using sealed secrets

After sealing a secret, it can be used inside kustomize deployments. While deploying, kluctl will look for resources
included from `kustomization.yaml` which are not existent but for which a file with a `.sealme` extension exists. If such
a file is found, the appropriate sealed secrets is located based on the
[outputPattern](#outputpattern-and-location-of-stored-sealed-secrets).

An example `kustomization.yaml`:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
# please note that we do not specify the .sealme suffix here
- db-secrets.yaml
- my-deployments.yaml
```

## outputPattern and location of stored sealed secrets

It is possible to override the output pattern in the root [deployment project]({{< ref "docs/reference/deployments" >}}).
The output pattern must be a template string that is rendered with the full
[templating context]({{< ref "docs/reference/templating" >}}) available for the deployment.yaml.

When manually specifying the outputPattern, ensure that it works well with multiple clusters and targets. You can
for example use the `{{ target.name }}` and `{{ cluster.name }}` inside the outputPattern.

```yaml
# deployment.yaml in root directory
sealedSecrets:
  outputPattern: "{{ cluster.name }}/{{ target.name }}"
```

The default outputPattern is simply `{{ target.name }}`, which should work well in most cases.

The final storage location for the sealed secret is:

`<base_dir>/<rendered_output_pattern>/<relative_sealme_file_dir>/<file_name>`

with:
* `base_dir`: The base directory for sealed secrets, which defaults to to the subdirectory `.sealed-secrets` in the kluctl project root
  diretory.
* `rendered_output_pattern`: The rendered outputPattern as described above.
* `relative_sealme_file_dir`: The relative path from the deployment root directory.
* `file_name`: The filename of the sealed secret, excluding the `.sealme` extension.

## Content Hashes and re-sealing
Sealed secrets are stored together with hashes of all individual secret entries. These hashes are then used to avoid
unnecessary re-sealing in future [seal]({{< ref "docs/reference/commands/seal" >}}) invocations. If you want to force re-sealing, use the
[--force-reseal]({{< ref "docs/reference/commands/seal" >}}) option.

Hashing of secrets is done with bcrypt and the cluster id as salt. The cluster id is currently defined as the sha256 hash
of the cluster CA certificate. This will cause re-sealing of all secrets in case a cluster is set up from scratch
(which causes key from the sealed secrets operator to get wiped as well).

## Clusters and namespaces
Sealed secrets are usually only decryptable by one cluster, simply because each cluster has its own set of randomly
generated public/private key pairs. This means, that a secret that was sealed for your production cluster can't be
unsealed on your test cluster.

In addition, sealed secrets can be bound to a single namespace, making them undecryptable for any other namespace.
To limit a sealed secret to a namespace, simply fill the `metadata.namespace` field of the input secret (which is in
the `.sealme` file). This way, the sealed secret can only be deployed to a single namespace.

You can also use [Scopes](https://github.com/bitnami-labs/sealed-secrets#scopes) to lift/limit restrictions.

## Using reflectors/replicators
In case a sealed secrets needs to be deployed to more than one namespace, some form of replication must be used. You'd
then seal the secret for a single namespace and use a reflection/replication controller to reflect the unsealed secret
into one or multiple other namespaces. Example controllers that can accomplish this are the
[Mittwald kubernetes-reflector](https://github.com/mittwald/kubernetes-replicator) and the
[Emberstack Kubernetes Reflector](https://github.com/emberstack/kubernetes-reflector).

Consider the following example (using the Mittwald replicator):
```yaml
kind: Secret
apiVersion: v1
metadata:
  name: db-secrets
  namespace: {{ my.namespace.variable }}
  annotations:
    replicator.v1.mittwald.de/replicate-to: '{{ my.namespace.variable }}-.*'
stringData:
  DB_URL: {{ secrets.database.url }}
  DB_USERNAME: {{ secrets.database.username }}
  DB_PASSWORD: {{ secrets.database.password }}
```

The above example would cause automatic replication into every namespace that matches the replicate-to pattern.

Please watch out for security implications. In the above example, everyone who has the right to create a namespace that
matches the pattern will get access to the secret.
