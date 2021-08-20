# Sealed Secrets integration

kluctl has an integration for [sealed secrets](https://github.com/bitnami-labs/sealed-secrets), allowing you to
securely store secrets for multiple target clusters and/or environments inside version control.

The integration consists of two parts:
1. Sealing of secrets
1. Automatically choosing and deploying the correct secrets for a target

# Requirements

The cluster must have the [sealed-secrets operator](https://github.com/bitnami-labs/sealed-secrets) installed. This
can either be done through the [bootstrap command](./commands.md#bootstrap) or through some other/custom means.

# Sealing of .sealme files

Sealing is done via the [seal command](./commands.md#seal). It must be done before the actual deployment is performed.
The `seal` command recursively searches for files that end with `.sealme`, renders them with the
[Jinja2 templating](./jinja2-templating.md) engine and then seals them by invoking `kubeseal` (part of 
[sealed secrets](https://github.com/bitnami-labs/sealed-secrets)).

The `.sealme` files itself have to be [Kubernetes Secrets](https://kubernetes.io/docs/concepts/configuration/secret/),
but without any actual secret data inside. The secret data is referenced via Jinja2 variables and is expected to be
provided only at the time of sealing. This means, that the sensitive secret data must only be in clear text while sealing.
Afterwards, the sensitive data can be removed locally, and the sealed secrets can be added to version control.

Example file (the name could be for example `db-secrets.yml.sealme`):
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

While sealing, the full Jinja2 Context (same as in [Jinja2 Templating](./jinja2-templating.md)) is available.
Additionally, the global `secrets` object/variable is available which contains the sensitive secrets.

## Secret Sources

Secrets are only loaded while sealing. Available secret sets and sources are configured via
[.kluctl.yml](./kluctl_project.md#supported-sources). The secrets used per target are configured via the
[secrets config](./kluctl_project.md#secretsconfig) of the targets.

# Using sealed secrets

After sealing a secret, it can be used inside kustomize deployments. While deploying, kluctl will look for resources
included from `kustomization.yml` which are not existent but for which a file with a `.sealme` extension exists. If such
a file is found, the appropriate sealed secrets is located based on the
[outputPattern](#outputpattern-and-location-of-stored-sealed-secrets).

An example `kustomization.yml`:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
# please note that we do not specify the .sealme suffix here
- db-secrets.yml
- my-deployments.yml
```

# outputPattern and location of stored sealed secrets
The root [deployment project](./deployments.md) must specify the outputPattern to use when storing/loading
sealed secrets. The output pattern must be a Jinja2 Template that is rendered with the full
[Jinja2 Context](./jinja2-templating.md) available for the deployment.yml.

This template usually at least contains the cluster name. If your deployment can be deployed multiple times to the
same cluster (via different targets), you should also add something to differentiate the targets.

As an example, `{{ cluster.name }}/{{ args.environment }}` works well, assuming that you differentiate targets via
`args.environment`.

The final storage location for the sealed secret is:

`<base_dir>/<deployment_name>/<rendered_output_pattern>/<relative_sealme_file_dir>/<file_name>`

with:
* `base_dir`: The base directory for sealed secrets is configured in the [.kluctl.yml](./kluctl_project.md#sealedsecrets) config
file. If not specified, the base directory defaults to the subdirectory `.sealed-secrets` in the kluctl project root
diretory.
* `deployment_name`: The deployment name, which defaults to the kluctl project directories base name. It can also be
overridden with [--deployment-name](./commands.md#--deployment-name-text).
* `rendered_output_pattern`: The rendered outputPattern as described above.
* `relative_sealme_file_dir`: The relative path from the deployment root directory.
* `file_name`: The filename of the sealed secret, excluding the `.sealme` extension.

# Content Hashes and re-sealing
Sealed secrets are stored together with hashes of all individual secret entries. These hashes are then used to avoid
unnecessary re-sealing in future [seal](./commands.md#seal) invocations. If you want to force re-sealing, use the
[--force-reseal](./commands.md#--force-reseal) option.

# Clusters and namespaces
Sealed secrets are usually only decryptable by one cluster, simply because each cluster has its own set of randomly
generated public/private key pairs. This means, that a secret that was sealed for your production cluster can't be
unsealed on your test cluster.

In addition, sealed secrets can be bound to a single namespace, making them undecryptable for any other namespace.
To limit a sealed secret to a namespace, simply fill the `metadata.namespace` field of the input secret (which is in
the `.sealme` file). This way, the sealed secret can only be deployed to a single namespace.

You can also use [Scopes](https://github.com/bitnami-labs/sealed-secrets#scopes) to lift/limit restrictions.

# Using reflectors/replicators
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
