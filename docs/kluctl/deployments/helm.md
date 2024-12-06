<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Helm Integration"
linkTitle: "Helm Integration"
weight: 4
description: >
    How Helm is integrated into Kluctl.
---
-->

# Helm Integration

kluctl offers a simple-to-use Helm integration, which allows you to reuse many common third-party Helm Charts.

The integration is split into 2 parts/steps/layers. The first is the management and pulling of the Helm Charts, while
the second part handles configuration/customization and deployment of the chart.

It is recommended to pre-pull Helm Charts with [`kluctl helm-pull`](../commands/helm-pull.md), which will store the
pulled charts inside `.helm-charts` of the project directory. It is however also possible (but not
recommended) to skip the pre-pulling phase and let kluctl pull Charts on-demand.

When pre-pulling Helm Charts, you can also add the resulting Chart contents into version control. This is actually
recommended as it ensures that the deployment will always behave the same. It also allows pull-request based reviews
on third-party Helm Charts.

## How it works

Helm charts are not directly installed via Helm. Instead, kluctl renders the Helm Chart into a single file and then
hands over the rendered yaml to [kustomize](https://kustomize.io/). Rendering is done in combination with a provided
`helm-values.yaml`, which contains the necessary values to configure the Helm Chart.

The resulting rendered yaml is then referred by your `kustomization.yaml`, from which point on the
[kustomize integration](./kustomize.md) 
takes over. This means, that you can perform all desired customization (patches, namespace override, ...) as if you
provided your own resources via yaml files.

### Helm hooks

[Helm Hooks](https://helm.sh/docs/topics/charts_hooks/) are implemented by mapping them 
to [kluctl hooks](./hooks.md), based on the following mapping table:

| Helm hook     | kluctl hook         |
|---------------|---------------------|
| pre-install   | pre-deploy-initial  |
| post-install  | post-deploy-initial |
| pre-delete    | Not supported       |
| post-delete   | Not supported       |
| pre-upgrade   | pre-deploy-upgrade  |
| post-upgrade  | post-deploy-upgrade |
| pre-rollback  | Not supported       |
| post-rollback | Not supported       |
| test          | Not supported       |

Please note that this is a best effort approach and not 100% compatible to how Helm would run hooks.

## helm-chart.yaml

The `helm-chart.yaml` defines where to get the chart from, which version should be pulled, the rendered output file name,
and a few more Helm options. After this file is added to your project, you need to invoke the `helm-pull` command
to pull the Helm Chart into your local project. It is advised to put the pulled Helm Chart into version control, so
that deployments will always be based on the exact same Chart (Helm does not guarantee this when pulling).

Example `helm-chart.yaml`:

```yaml
helmChart:
  repo: https://charts.bitnami.com/bitnami
  chartName: redis
  chartVersion: 12.1.1
  updateConstraints: ~12.1.0
  skipUpdate: false
  skipPrePull: false
  releaseName: redis-cache
  namespace: "{{ my.jinja2.var }}"
  output: helm-rendered.yaml # this is optional
```

When running the `helm-pull` command, it will search for all `helm-chart.yaml` files in your project and then pull the
chart from the specified repository with the specified version. The pull chart will then be located in the sub-directory
`charts` below the same directory as the `helm-chart.yaml`

The same filename that was specified in `output` must then be referred in a `kustomization.yaml` as a normal local
resource. If `output` is omitted, the default value `helm-rendered.yaml` is used and must also be referenced in
`kustomization.yaml`.

`helmChart` inside `helm-chart.yaml` supports the following fields:

### repo
The url to the Helm repository where the Helm Chart is located. You can use hub.helm.sh to search for repositories and
charts and then use the repos found there.

OCI based repositories are also supported, for example:
```yaml
helmChart:
  repo: oci://r.myreg.io/mycharts/pepper
  chartVersion: 1.2.3
  releaseName: pepper
  namespace: pepper
```

### path
As alternative to `repo`, you can also specify `path`. The path must point to a local Helm Chart that is relative to the
`helm-chart.yaml`. The local Chart must reside in your Kluctl project.

When `path` is specified, `repo`, `chartName`, `chartVersion` and `updateContrainsts` are not allowed.

### chartName
The name of the chart that can be found in the repository.

### chartVersion
The version of the chart. Must be a valid semantic version.

### git
Instead of using `repo` for OCI/Helm registries or `path` for local charts, you can also pull Charts from Git repositories.
This is helpful in cases where you don't want to publish a chart to a registry or as page, e.g. because of the overhead or other internal restrictions.
You have to set the `url` as well as the `branch`, `tag` or `commit`. If the chart itself is in a sub directory, you can also specify a `subDir`:

```yaml
helmChart:
  git:
    url: https://github.com/mycharts/salt
    ref:
      branch: main
      #tag: v1.0.0 -- branch, tag and commit are mutually exclusive
      #commit: 015244630b53eb69d77858e5587641b741e91706 -- branch, tag and commit are mutually exclusive
    subDir: charts/path/to/chart
  releaseName: salt
  namespace: salt
```
In order to be able to use the `helm-update` command, the branch or tag has to be semantic. If this is not the case, the update is skipped.

### updateConstraints
Specifies version constraints to be used when running [helm-update](../commands/helm-update.md). See
[Checking Version Constraints](https://github.com/Masterminds/semver#checking-version-constraints) for details on the
supported syntax.

If omitted, Kluctl will filter out pre-releases by default. Use a `updateConstraints` like `~1.2.3-0` to enable
pre-releases.

### skipUpdate
If set to `true`, skip this Helm Chart when the [helm-update](../commands/helm-update.md) command is called.
If omitted, defaults to `false`.

### skipPrePull
If set to `true`, skip pre-pulling of this Helm Chart when running [helm-pull](../commands/helm-pull.md). This will
also enable pulling on-demand when the deployment project is rendered/deployed.

### releaseName
The name of the Helm Release.

### namespace
The namespace that this Helm Chart is going to be deployed to. Please note that this should match the namespace
that you're actually deploying the kustomize deployment to. This means, that either `namespace` in `kustomization.yaml`
or `overrideNamespace` in `deployment.yaml` should match the namespace given here. The namespace should also be existing
already at the point in time when the kustomize deployment is deployed.

### output
This is the file name into which the Helm Chart is rendered into. Your `kustomization.yaml` should include this same
file. The file should not be existing in your project, as it is created on-the-fly while deploying.

### skipCRDs
If set to `true`, kluctl will pass `--skip-crds` to Helm when rendering the deployment. If set to `false` (which is
the default), kluctl will pass `--include-crds` to Helm.

## helm-values.yaml
This file should be present when you need to pass custom Helm Value to Helm while rendering the deployment. Please
read the documentation of the used Helm Charts for details on what is supported.

## Updates to helm-charts
In case a Helm Chart needs to be updated, you can either do this manually by replacing the [chartVersion](#chartversion)
value in `helm-chart.yaml` and the calling the [helm-pull](../commands/helm-pull.md) command or by simply invoking
[helm-update](../commands/helm-update.md) with `--upgrade` and/or `--commit` being set.

## Private Repositories
It is also possible to use private chart repositories and private OCI registries. There are multiple options to
provide credentials to Kluctl.

### Use `helm repo add --username xxx --password xxx` before
Kluctl will try to find known repositories that are managed by the Helm CLI and then try to reuse the credentials of
these. The repositories are identified by the URL of the repository, so it doesn't matter what name you used when you
added the repository to Helm. The same method can be used for client certificate based authentication (`--key-file`
in `helm repo add`).

### Use `helm registry login --username xxx --password xxx` for OCI registries
The same as for `helm repo add` applies here, except that authentication entries are matched by hostname.


### Use `docker login` for OCI registries
Kluctl tries to use credentials stored in `$HOME/.docker/config.json` as well, so
[`docker login`](https://docs.docker.com/engine/reference/commandline/login/) will also allow Kluctl to authenticate
against OCI registries.

### Use the --helm-xxx and --registry-xxx arguments of Kluctl sub-commands
All [commands](../commands/README.md) that interact with Helm Chart repositories and OCI registries support the
[helm arguments](../commands/common-arguments.md#helm-arguments) and [registry arguments](../commands/common-arguments.md#registry-arguments)
to specify authentication per repository and/or OCI registry.

⚠️DEPRECATION WARNING ⚠️
Previous versions (prior to v2.22.0) of Kluctl supported managing Helm credentials via `credentialsId` in `helm-chart.yaml`.
This is deprecated now and will be removed in the future. Please switch to hostname/registry-name based authentication
instead. See [helm arguments](../commands/common-arguments.md#helm-arguments) for details.

### Use environment variables to specify authentication
You can also use environment variables to specify Helm Chart repository authentication. For OCI based registries, see
[OCI authentication](./oci.md#authentication) for details.

The following environment variables are supported:

1. `KLUCTL_HELM_HOST`: Specifies the host name of the repository to match before the specified credentials are considered.
2. `KLUCTL_HELM_PATH`: Specifies the path to match before the specified credentials are considered. If omitted, credentials are applied to all matching hosts. Can contain wildcards.
3. `KLUCTL_HELM_USERNAME`: Specifies the username.
4. `KLUCTL_HELM_PASSWORD`: Specifies the password.
5. `KLUCTL_HELM_INSECURE_SKIP_TLS_VERIFY`: If set to `true`, Kluctl will skip TLS verification for matching repositories.
6. `KLUCTL_HELM_PASS_CREDENTIALS_ALL`: If set to `true`, Kluctl will instruct Helm to pass credentials to all domains. See https://helm.sh/docs/helm/helm_repo_add/ for details.
7. `KLUCTL_HELM_CERT_FILE`: Specifies the client certificate to use while connecting to the matching repository.
8. `KLUCTL_HELM_KEY_FILE`: Specifies the client key to use while connecting to the matching repository.
9. `KLUCTL_HELM_CA_FILE`: Specifies CA bundle to use for TLS/https verification.

Multiple credential sets can be specified by including an index in the environment variable names, e.g.
`KLUCTL_HELM_1_HOST=host.org`, `KLUCTL_HELM_1_USERNAME=my-user` and `KLUCTL_HELM_1_PASSWORD=my-password` will apply
the given credential to all repositories with the host `host.org`, while `KLUCTL_HELM_2_HOST=other.org`,
`KLUCTL_HELM_2_USERNAME=my-other-user` and `KLUCTL_HELM_2_PASSWORD=my-other-password` will apply the other credentials
to the `other.org` repository.

### Credentials when using the kluctl-controller
In case you want to use the same Kluctl deployment via the [kluctl-controller](../../gitops/README.md), you have to
configure Helm and OCI credentials via [`spec.credentials`](../../gitops/spec/v1beta1/kluctldeployment.md#credentials).

## Templating

Both `helm-chart.yaml` and `helm-values.yaml` are rendered by the [templating engine](../templating) before they
are actually used. This means, that you can use all available Jinja2 variables at that point, which can for example be
seen in the above `helm-chart.yaml` example for the namespace.

There is however one exception that leads to a small limitation. When `helm-pull` reads the `helm-chart.yaml`, it does
NOT render the file via the templating engine. This is because it can not know how to properly render the template as it
does have no information about targets (there are no `-t` arguments set) at that point.

This exception leads to the limitation that the `helm-chart.yaml` MUST be valid yaml even in case it is not rendered
via the templating engine. This makes using control statements (if/for/...) impossible in this file. It also makes it
a requirement to use quotes around values that contain templates (e.g. the namespace in the above example).

`helm-values.yaml` is not subject to these limitations as it is only interpreted while deploying.
