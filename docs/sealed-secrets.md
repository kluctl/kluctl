# Sealed Secrets integration

kluctl has an integration for [sealed secrets](https://github.com/bitnami-labs/sealed-secrets), allowing you to
securely store secrets for multiple target clusters and/or environments inside version control.

The integration consists of two parts:
1. Sealing of secrets
1. Automatically choosing and deploying the correct secrets for a target environment/cluster

# Requirements

The [bootstrap](../bootstrap) deployment has to be deployed to your cluster, as it includes the sealed secrets
controller. If the controller is not deployed, kluctl won't know which public keys to use for sealing.

You can also provide your own version of the sealed-secrets operator, it however must be located in the kube-system
namespace for now.

# Sealing of .sealme files

Sealing is done via the `seal` command. It must be done before the actual deployment is performed.
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

While sealing, only a very small set of variables is available while template rendering happens. To be specific, only
the variables and arguments found in `sealme-conf.yml` are available. Additionally, the `secrets` object/variable is
available which contains the sensitive secrets.

Variables defines in `sealme-conf.yml` must to some degree mirror what is defined in the actual deployment. Especially
when it comes to namespace names, which must match with what is deployed later. To accomplish this, it might make sense
to have some commonly used variable yamls which is used in the [deployment](./jinja2-templating.md#vars-from-deploymentyml)
and [while sealing](#vars).

# Using sealed secrets

After sealing a secret, it can be used inside kustomize deployments. While deploying, kluctl will look for resources
included from `kustomization.yml` which are not existent but for which a file with a `.sealme` extension exists. If such
a file is found, the appropriate sealed secrets is located based on the
[outputPattern](#outputpattern-and-location-of-stored-sealed-secrets) of the `sealme-conf.yml` configuration.

An example `kustomization.yml`:
```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
# please note that we do not specify the .sealme suffix here
- db-secrets.yml
- my-deployments.yml
```

# sealme-conf.yml

The Sealed Secrets integration is configured via a `sealme-conf.yml`, which can be present at any directory level in your
deployment project. It is always the one active that has the shortest distance to the `.sealme` file that is being
sealed.

The `sealme-conf.yml` is NOT fully rendered via the templating engine. See [Jinja2 Templating](#jinja2-templating) for
details on what and how it is rendered.

Sealing is performed per cluster (or only for a single cluster if the `-N` argument is given). For each cluster that is
processed, the `secretsMatrix` is evaluated. Then, each matrix entry where all [rules](#rules) result in `True` will
result in an independent run of sealing.

The `sealme-conf.yml` offers the following fields:

### outputPattern
See [below](#outputpattern-and-location-of-stored-sealed-secrets)

### vars
This field can be used to provide variables while rendering `.sealme` files. The format of `vars` the same as
in [deployment vars](./deployments.md#vars). You can also reuse the same variable yaml as in `deployment.yml`, the
requirement is however that the included variables yaml only references Jinja2 variables which exists in the sealing
context. This means for example, that [-a](./commands.md#-a---arg-text) based arguments are not available, but values
from `argsMatrix` are available instead.

Rendering of variables is performed the same way as in `deployment.yml`, meaning that the second entry can also use
values from the first and so on.

See [Jinja2 Templating](#jinja2-templating) for more details.

Example vars:
```yaml
vars:
- file: vars1.yml
- values:
    var1: value1
    var2:
      var3: value3
```

### secretsMatrix
A list of secret matrix entries. Each entry defines where secrets are loaded from and for which set of `args` to perform
sealing runs.

The combination of available clusters (as defined by the [cluster config](./cluster-config.md)), `secretsMatrix` entries
and `argsMatrix` entries defines the sealing runs to be tried.

Consider the following example matrix and assume there are 4 clusters (devops, test, staging, prod):

```yaml
secretsMatrix:
- rules:
  - "{{ cluster.name == cluster_name }}"
  secrets:
    - path: .secrets-non-prod.yml
  argsMatrix:
    - cluster_name: test.example.com
      environment: test
    - cluster_name: test.example.com
      environment: dev
- rules:
  - "{{ cluster.name == cluster_name }}"
  secrets:
    - path: .secrets-prod.yml
  argsMatrix:
    - cluster_name: staging.example.com
      environment: staging
    - cluster_name: prod.example.com
      environment: prod
```

This example will result in 16 runs (4 clusters * 2 `secretsMatrix` * `argsMatrix`). For each run, rules are evaluated,
which results in 4 runs being actually executed, each with different combinations of cluster and environment config.

Each `secretsMatrx` entry offers the following fields:

### rules
A list of strings which are rendered via Jinja2 templating. The same variables described in [Jinja2 Templates](#jinja2-templating) are
available while rendering these rules. Only if all rules result in rendering of `True`, the sealing run is executed for
the current cluster and `argsMatrix` entry.

### secrets
A list of sources to read from when filling the global `secrets` object described in [Jinja2 Templating](#jinja2-templating).
Different source types are supported, and more are going to be implemented in the future (feel free to create a pull request).

#### path
A simple local file based source. The path must be relative and multiple places are tried to find the file:
1. Relative to the `sealme-conf.yml`
2. The path provided via [--secrets-dir](./commands.md#--secrets-dir-directory)
3. The deployment root directory

Example:
```yaml
secretsMatrix:
- rules:
  - "{{ cluster.name == cluster_name }}"
  secrets:
    - path: .secrets-non-prod.yml
  argsMatrix:
    - cluster_name: test.example.com
      environment: test
```

### argsMatrix

#### passwordstate
[Passwordstate](https://www.clickstudios.com.au/passwordstate.aspx) integration. It requires API access rights and will
prompt for login information while sealing. The password entry must have a field with the name given in `passwordField`
(defaults to `GenericField1`) which contains the yaml file with the actual secrets.

Example:
```yaml
secretsMatrix:
- rules:
  - "{{ cluster.name == cluster_name }}"
  secrets:
    - passwordstate:
        host: passwordstate.example.com
        passwordList: /My/Path/To/The/List/name-of-the-list
        passwordTitle: title-of-the-password
        passwordField: field-name # This is optional and defaults to GenericField1
  argsMatrix:
    - cluster_name: test.example.com
      environment: test
```

# Jinja2 Templating

Sealing uses Jinja2 Templating in the same way as described in [Jinja2 Templating](./jinja2-templating.md). The main
difference is that the global `args` variable is not filled with values from `-a`, but with the values found in
[argsMatrix](#argsmatrix) corresponding to the current sealing run. Handling and rendering of [vars](#vars) is
equivalent to how it's done in [deployment.yml](./deployments.md#vars).

Additionally, the global variable `secrets` is available while rendering `.sealme` files. It contains all merged secrets
loaded from [secrets](#secrets).

The `sealme-conf.yml` is not fully rendered. Instead, many individual fields are rendered on their own. These include:
1. All values found in [secrets](#secrets) (not the content of the secret files, but the source definitions). This means
that you can also parametrize loading of secrets (e.g. by putting the environment name into the file to load).

# outputPattern and location of stored sealed secrets
The `sealme-conf.yml` configuration must specify the `outputPattern` used to store and find sealed secrets. The output
pattern is a Jinja2 template that is rendered in two different places:

1. It is rendered each time a `.sealme` file is sealed. The result is then used to build the path where to store
   the resulting sealed secret.
1. While processing a `deploy`, `diff`, `list-images` or `render` command. kluctl will render the `outputPattern`
   of the corresponding `sealme-conf.yml` and use it to find the correct sealed secret, which can then be used via
   kustomize.
   
Please note that in both cases, the source of the available Jinja2 variables is different. In the first case, the
variables available are the ones described in [Jinja2 Templating](#jinja2-templating). In the second case, all variables
are available that are also available when [normal rendering](./jinja2-templating.md) for deployments happens. However,
you must ensure that even though these are different in theory, the result should match and be compatible between both. 
This means, that you must only use variables in `outputPattern` that are available in both scenarios and also give the
same results. Failing to do so will result in undefined behaviour.

The base directory for sealed secrets is by default `.sealed-secrets` and is below the deployment project root. The
output path that is rendered from `outputPattern` is then used in combination with the relative base path of the
`.sealme` file. All three elements combined give the full path of the storing location of the sealed secret.

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
