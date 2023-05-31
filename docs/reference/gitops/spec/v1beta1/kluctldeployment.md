<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: KluctlDeployment
linkTitle: KluctlDeployment
description: KluctlDeployment documentation
weight: 20
---
-->

# KluctlDeployment

The `KluctlDeployment` API defines a deployment of a [target](../../../kluctl-project/targets)
from a [Kluctl Project](../../../kluctl-project).

## Example

```yaml
apiVersion: gitops.kluctl.io/v1beta1
kind: KluctlDeployment
metadata:
  name: microservices-demo-prod
spec:
  interval: 5m
  source:
    url: https://github.com/kluctl/kluctl-examples.git
    path: "./microservices-demo/3-templating-and-multi-env/"
  timeout: 2m
  target: prod
  context: default
  prune: true
  delete: true
```

In the above example a KluctlDeployment is being created that defines the deployment based on the Kluctl project.

The deployment is performed every 5 minutes. It will deploy the `prod`
[target](../../../kluctl-project/targets) and then prune orphaned objects afterward.

When the KluctlDeployment gets deleted, `delete: true` will cause the controller to actually delete the target
resources.

It uses the `default` context provided by the default service account and thus overrides the context specified in the
target definition.

## Spec fields

### source

The KluctlDeployment `spec.source` specifies the source repository to be used. Example:

```yaml
apiVersion: gitops.kluctl.io/v1beta1
kind: KluctlDeployment
metadata:
  name: example
spec:
  source:
    url: https://github.com/kluctl/kluctl-examples.git
    path: path/to/project
    secretRef:
      name: git-credentials
    ref:
      branch: my-branch
  ...
```

The `url` specifies the git clone url. It can either be a https or a git/ssh url. Git/Ssh url will require a secret
to be provided with credentials.

The `path` specifies the sub-directory where the Kluctl project is located.

The `ref` provides the Git reference to be used. It can either be a branch or a tag.

See [Git authentication](#git-authentication) for details on authentication.

### interval
See [Reconciliation](#reconciliation).

### suspend
See [Reconciliation](#reconciliation).

### target
`spec.target` specifies the target to be deployed. It must exist in the Kluctl projects
[kluctl.yaml targets](../../../kluctl-project/targets) list.

This field is optional and can be omitted if the referenced Kluctl project allows deployments without targets.

### targetNameOverride
`spec.targetNameOverride` will set or override the name of the target. This is equivalent to passing
`--target-name-override` to `kluctl deploy`.

### context
`spec.context` will override the context used while deploying. This is equivalent to passing `--context` to
`kluctl deploy`.

### deployMode
By default, the operator will perform a full deployment, which is equivalent to using the `kluctl deploy` command.
As an alternative, the controller can be instructed to only perform a `kluctl poke-images` command. Please
see [poke-images](../../../commands/poke-images.md) for details on the command. To do so, set `spec.deployMode`
field to `poke-images`.

Example:
```
apiVersion: gitops.kluctl.io/v1beta1
kind: KluctlDeployment
metadata:
  name: microservices-demo-prod
spec:
  interval: 5m
  source:
    url: https://github.com/kluctl/kluctl-examples.git
    path: "./microservices-demo/3-templating-and-multi-env/"
  timeout: 2m
  target: prod
  context: default
  deployMode: poke-images
```

### prune

To enable pruning, set `spec.prune` to `true`. This will cause the controller to run `kluctl prune` after each
successful deployment.

### delete

To enable deletion, set `spec.delete` to `true`. This will cause the controller to run `kluctl delete` when the
KluctlDeployment gets deleted.

### args
`spec.args` is an object representing [arguments](../../../kluctl-project/#args)
passed to the deployment. Example:

```yaml
apiVersion: gitops.kluctl.io/v1beta1
kind: KluctlDeployment
metadata:
  name: example
spec:
  interval: 5m
  source:
    url: https://github.com/kluctl/kluctl-examples.git
    path: "./microservices-demo/3-templating-and-multi-env/"
  timeout: 2m
  target: prod
  context: default
  args:
    arg1: value1
    arg2: value2
    arg3:
      k1: v1
      k2: v2
```

The above example is equivalent to calling `kluctl deploy -t prod -a arg1=value1 -a arg2=value2`.

### images
`spec.images` specifies a list of fixed images to be used by
[`image.get_image(...)`](../../../deployments/images#imagesget_image). Example:

```
apiVersion: gitops.kluctl.io/v1beta1
kind: KluctlDeployment
metadata:
  name: example
spec:
  interval: 5m
  source:
    url: https://example.com
  timeout: 2m
  target: prod
  images:
    - image: nginx
      resultImage: nginx:1.21.6
      namespace: example-namespace
      deployment: Deployment/example
    - image: registry.gitlab.com/my-org/my-repo/image
      resultImage: registry.gitlab.com/my-org/my-repo/image:1.2.3
```

The above example will cause the `images.get_image("nginx")` invocations of the `example` Deployment to return
`nginx:1.21.6`. It will also cause all `images.get_image("registry.gitlab.com/my-org/my-repo/image")` invocations
to return `registry.gitlab.com/my-org/my-repo/image:1.2.3`.

The fixed images provided here take precedence over the ones provided in the
[target definition](../../../kluctl-project/targets#images).

`spec.images` is equivalent to calling `kluctl deploy -t prod --fixed-image=nginx:example-namespace:Deployment/example=nginx:1.21.6 ...`
and to `kluctl deploy -t prod --fixed-images-file=fixed-images.yaml` with `fixed-images.yaml` containing:

```yaml
images:
- image: nginx
  resultImage: nginx:1.21.6
  namespace: example-namespace
  deployment: Deployment/example
- image: registry.gitlab.com/my-org/my-repo/image
  resultImage: registry.gitlab.com/my-org/my-repo/image:1.2.3
```

### dryRun
`spec.dryRun` is a boolean value that turns the deployment into a dry-run deployment. This is equivalent to calling
`kluctl deploy -t prod --dry-run`.

### noWait
`spec.noWait` is a boolean value that disables all internal waiting (hooks and readiness). This is equivalent to calling
`kluctl deploy -t prod --no-wait`.

### forceApply
`spec.forceApply` is a boolean value that causes kluctl to solve conflicts via force apply. This is equivalent to calling
`kluctl deploy -t prod --force-apply`.

### replaceOnError and forceReplaceOnError
`spec.replaceOnError` and `spec.forceReplaceOnError` are both boolean values that cause kluctl to perform a replace
after a failed apply. `forceReplaceOnError` goes a step further and deletes and recreates the object in question.
These are equivalent to calling `kluctl deploy -t prod --replace-on-error` and `kluctl deploy -t prod --force-replace-on-error`.

### abortOnError
`spec.abortOnError` is a boolean value that causes kluctl to abort as fast as possible in case of errors. This is equivalent to calling
`kluctl deploy -t prod --abort-on-error`.

### includeTags, excludeTags, includeDeploymentDirs and excludeDeploymentDirs
`spec.includeTags` and `spec.excludeTags` are lists of tags to be used in inclusion/exclusion logic while deploying.
These are equivalent to calling `kluctl deploy -t prod --include-tag <tag1>` and `kluctl deploy -t prod --exclude-tag <tag2>`.

`spec.includeDeploymentDirs` and `spec.excludeDeploymentDirs` are lists of relative deployment directories to be used in
inclusion/exclusion logic while deploying. These are equivalent to calling `kluctl deploy -t prod --include-tag <tag1>`
and `kluctl deploy -t prod --exclude-tag <tag2>`.

## Reconciliation

The KluctlDeployment `spec.interval` tells the controller at which interval to try reconciliations.
The interval time units are `s`, `m` and `h` e.g. `interval: 5m`, the minimum value should be over 60 seconds.

At each reconciliation run, the controller will check if any rendered objects have been changes since the last
deployment and then perform a new deployment if changes are detected. Changes are tracked via a hash consisting of
all rendered objects.

To enforce periodic full deployments even if nothing has changed, `spec.deployInterval` can be used to specify an
interval at which forced deployments must be performed by the controller.

The KluctlDeployment reconciliation can be suspended by setting `spec.suspend` to `true`.

The controller can be told to reconcile the KluctlDeployment outside of the specified interval
by annotating the KluctlDeployment object with `kluctl.io/request-reconcile`.

On-demand reconciliation example:

```bash
kubectl annotate --overwrite kluctldeployment/microservices-demo-prod kluctl.io/request-reconcile="$(date +%s)"
```

Similarly, a deployment can be forced even if the source has not changed by using the  `kluctl.io/request-deploy`
annotation:

```bash
kubectl annotate --overwrite kluctldeployment/microservices-demo-prod kluctl.io/request-deploy="$(date +%s)"
```


## Kubeconfigs and RBAC

As Kluctl is meant to be a CLI-first tool, it expects a kubeconfig to be present while deployments are
performed. The controller will generate such kubeconfigs on-the-fly before performing the actual deployment.

The kubeconfig can be generated from 3 different sources:
1. The default impersonation service account specified at controller startup (via `--default-service-account`)
2. The service account specified via `spec.serviceAccountName` in the KluctlDeployment
3. The secret specified via `spec.kubeConfig` in the KluctlDeployment.

The behavior/functionality of 1. and 2. is comparable to how the [kustomize-controller](https://fluxcd.io/docs/components/kustomize/kustomization/#role-based-access-control)
handles impersonation, with the difference that a kubeconfig with a "default" context is created in-between.

`spec.kubeConfig` will simply load the kubeconfig from `data.value` of the specified secret.

Kluctl [targets](../../../kluctl-project/targets) specify a context name that is expected to
be present in the kubeconfig while deploying. As the context found in the generated kubeconfig does not necessarily
have the correct name, `spec.context` can be used to while deploying. This is especially useful
when using service account based kubeconfigs, as these always have the same context with the name "default".

Here is an example of a deployment that uses the service account "prod-service-account" and overrides the context
appropriately (assuming the Kluctl cluster config for the given target expects a "prod" context):

```yaml
apiVersion: gitops.kluctl.io/v1beta1
kind: KluctlDeployment
metadata:
  name: example
  namespace: kluctl-system
spec:
  interval: 10m
  source:
    url: https://github.com/kluctl/kluctl-examples.git
    path: "./microservices-demo/3-templating-and-multi-env/"
  target: prod
  serviceAccountName: prod-service-account
  context: default
```

## Git authentication

The `spec.source` can optionally specify a `spec.source.secretRef` (see [here](#source)) which must point to an existing
secret (in the same namespace) containing Git credentials.

### Basic access authentication

To authenticate towards a Git repository over HTTPS using basic access
authentication (in other words: using a username and password), the referenced
Secret is expected to contain `.data.username` and `.data.password` values.

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: basic-access-auth
type: Opaque
data:
  username: <BASE64>
  password: <BASE64>
```

### HTTPS Certificate Authority

To provide a Certificate Authority to trust while connecting with a Git
repository over HTTPS, the referenced Secret can contain a `.data.caFile`
value.

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: https-ca-credentials
  namespace: default
type: Opaque
data:
  caFile: <BASE64>
```

### SSH authentication

To authenticate towards a Git repository over SSH, the referenced Secret is
expected to contain `identity` and `known_hosts` fields. With the respective
private key of the SSH key pair, and the host keys of the Git repository.

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: ssh-credentials
type: Opaque
stringData:
  identity: |
    -----BEGIN OPENSSH PRIVATE KEY-----
    ...
    -----END OPENSSH PRIVATE KEY-----
  known_hosts: |
    github.com ecdsa-sha2-nistp256 AAAA...
```

## Helm Repository authentication

Kluctl allows to [integrate Helm Charts](../../../deployments/helm.md) in two different ways.
One is to [pre-pull charts](../../../commands/helm-pull.md) and put them into version control,
making it unnecessary to pull them at deploy time. This option also means that you don't have to take any special care
on the controller side.

The other way is to let Kluctl pull Helm Charts at deploy time. In that case, you have to ensure that the controller
has the necessary access to the Helm repositories. To add credentials for authentication, set the `spec.helmCredentials`
field to a list of secret references:

### Basic access authentication

```yaml
apiVersion: gitops.kluctl.io/v1beta1
kind: KluctlDeployment
metadata:
  name: example
  namespace: kluctl-system
spec:
  interval: 10m
  source:
    url: https://github.com/kluctl/kluctl-examples.git
    path: "./microservices-demo/3-templating-and-multi-env/"
  target: prod
  serviceAccountName: prod-service-account
  context: default

  helmCredentials:
    - secretRef:
        name: helm-creds
---
apiVersion: v1
kind: Secret
metadata:
  name: helm-creds
  namespace: kluctl-system
stringData:
  url: https://example-repo.com
  username: my-user
  password: my-password
```

### TLS authentication

For TLS authentication, see the following example secret:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: helm-creds
  namespace: kluctl-system
data:
  certFile: <BASE64>
  keyFile: <BASE64>
  # NOTE: Can be supplied without the above values
  caFile: <BASE64>
```

### Disabling TLS verification

In case you need to disable TLS verification (not recommended!), add the key `insecureSkipTlsVerify` with the value
`"true"` (make sure it's a string, so surround it with `"`).

### Pass credentials

To enable passing of credentials to all requests, add the key `passCredentialsAll` with the value `"true"`.
This will pass the credentials to all requests, even if the hostname changes.

## Secrets Decryption

Kluctl offers a [SOPS Integration](../../../deployments/sops.md) that allows to use encrypted
manifests and variable sources in Kluctl deployments. Decryption by the controller is also supported and currently
mirrors how the [Secrets Decryption configuration](https://fluxcd.io/flux/components/kustomize/kustomization/#secrets-decryption)
of the Flux Kustomize Controller. To configure it in the `KluctlDeployment`, simply set the `decryption` field in the
spec:

```
apiVersion: gitops.kluctl.io/v1beta1
kind: KluctlDeployment
metadata:
  name: example
  namespace: kluctl-system
spec:
  decryption:
    provider: sops
    secretRef:
      name: sops-keys
  ...
```

The `sops-keys` Secret has the same format as in the
[Flux Kustomize Controller](https://fluxcd.io/flux/components/kustomize/kustomization/#decryption-secret-reference).

### AWS KMS with IRSA

In addition to the [AWS KMS Secret Entry](https://fluxcd.io/flux/components/kustomize/kustomization/#aws-kms-secret-entry)
in the secret and the [global AWS KMS](https://fluxcd.io/flux/components/kustomize/kustomization/#aws-kms)
authentication via the controller's service account, the Kluctl controller also supports using the IRSA role of the
impersonated service account of the `KluctlDeployment` (specified via `serviceAccountName` in the spec or
`--default-service-account`):

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kluctl-deployment
  namespace: kluctl-system
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::123456:role/my-irsa-enabled-role
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: kluctl-deployment
  namespace: kluctl-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  # watch out, don't use cluster-admin if you don't trust the deployment
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: kluctl-deployment
    namespace: kluctl-system
---
apiVersion: gitops.kluctl.io/v1beta1
kind: KluctlDeployment
metadata:
  name: example
  namespace: kluctl-system
spec:
  serviceAccountName: kluctl-deployment
  decryption:
    provider: sops
    # you can also leave out the secretRef if you don't provide addinional keys
    secretRef:
      name: sops-keys
  ...
```

## Status

When the controller completes a deployments, it reports the result in the `status` sub-resource.

A successful reconciliation sets the ready condition to `true`.

```yaml
status:
  conditions:
  - lastTransitionTime: "2022-07-07T11:48:14Z"
    message: "deploy: ok"
    reason: ReconciliationSucceeded
    status: "True"
    type: Ready
  lastDeployResult:
    ...
  lastPruneResult:
    ...
  lastValidateResult:
    ...
```

You can wait for the controller to complete a reconciliation with:

```bash
kubectl wait kluctldeployment/backend --for=condition=ready
```

A failed reconciliation sets the ready condition to `false`:

```yaml
status:
  conditions:
  - lastTransitionTime: "2022-05-04T10:18:11Z"
    message: target invalid-name not found in kluctl project
    reason: PrepareFailed
    status: "False"
    type: Ready
  lastDeployResult:
    ...
  lastPruneResult:
    ...
  lastValidateResult:
    ...
```

> **Note** that the lastDeployResult, lastPruneResult and lastValidateResult are only updated on a successful reconciliation.
