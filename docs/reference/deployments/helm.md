---
title: "Helm Integration"
linkTitle: "Helm Integration"
weight: 3
description: >
    How Helm is integrated into Kluctl.
---

kluctl offers a simple-to-use Helm integration, which allows you to reuse many common third-party Helm Charts.

The integration is split into 2 parts/steps/layers. The first is the management and pulling of the Helm Charts, while
the second part handles configuration/customization and deployment of the chart.

Pulled Helm Charts are meant to be added to version control to ensure proper speed and consistency.

## How it works

Helm charts are not directly deployed via Helm. Instead, kluctl renders the Helm Chart into a single file and then
hands over the rendered yaml to [kustomize](https://kustomize.io/). Rendering is done in combination with a provided
`helm-values.yaml`, which contains the necessary values to configure the Helm Chart.

The resulting rendered yaml is then referred by your `kustomization.yaml`, from which point on the
[kustomize integration]({{< ref "docs/reference/sealed-secrets#outputpattern-and-location-of-stored-sealed-secrets" >}}) 
takes over. This means, that you can perform all desired customization (patches, namespace override, ...) as if you
provided your own resources via yaml files.

### Helm hooks

[Helm Hooks](https://helm.sh/docs/topics/charts_hooks/) are implemented by mapping them 
to [kluctl hooks]({{< ref "./hooks" >}}), based on the following mapping table:

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
  skipUpdate: false
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

oci based repositories are also supported, for example:
```yaml
helmChart:
  repo: oci://r.myreg.io/mycharts/pepper
  chartName: pepper
  chartVersion: 1.2.3
  releaseName: pepper
  namespace: pepper
```

### chartName
The name of the chart that can be found in the repository.

### chartVersion
The version of the chart.

### skipUpdate
Skip this Helm Chart when the [helm-update]({{< ref "docs/reference/commands/helm-update" >}}) command is called.
If omitted, defaults to `false`.

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
value in `helm-chart.yaml` and the calling the [helm-pull]({{< ref "docs/reference/commands/helm-pull" >}}) command or by simply invoking
[helm-update]({{< ref "docs/reference/commands/helm-update" >}}) with `--upgrade` and/or `--commit` being set.

## Private Chart Repositories
It is also possible to use private chart repositories. There are currently two options to provide Helm Repository
credentials to Kluctl.

### Use `helm repo add --username xxx --password xxx` before
Kluctl will try to find known repositories that are managed by the Helm CLI and then try to reuse the credentials of
these. The repositories are identified by the URL of the repository, so it doesn't matter what name you used when you
added the repository to Helm. The same method can be used for client certificate based authentication (`--key-file`
in `helm repo add`).

### Use the --username/--password arguments in `kluctl helm-pull`
See the [helm-pull command]({{< ref "docs/reference/commands/helm-pull" >}}). You can control repository credentials
via `--username`, `--password` and `--key-file`. Each argument must be in the form `credentialsId:value`, where
the `credentialsId` must match the id specified in the `helm-chart.yaml`. Example:

```yaml
helmChart:
  repo: https://raw.githubusercontent.com/example/private-helm-repo/main/
  credentialsId: private-helm-repo
  chartName: my-chart
  chartVersion: 1.2.3
  releaseName: my-chart
  namespace: default
```

When credentialsId is specified, Kluctl will require you to specify `--username=private-helm-repo:my-username` and
`--password=private-helm-repo:my-password`. You can also specify a client-side certificate instead via
`--key-file=private-helm-repo:/path/to/cert`.

Multiple Helm Charts can use the same `credentialsId`.

Environment variables can also be used instead of arguments. See
[Environment Variables]({{< ref "docs/reference/commands/environment-variables" >}}) for details.

## Templating

Both `helm-chart.yaml` and `helm-values.yaml` are rendered by the [templating engine]({{< ref "docs/reference/templating" >}}) before they
are actually used. This means, that you can use all available Jinja2 variables at that point, which can for example be
seen in the above `helm-chart.yaml` example for the namespace.

There is however one exception that leads to a small limitation. When `helm-pull` reads the `helm-chart.yaml`, it does
NOT render the file via the templating engine. This is because it can not know how to properly render the template as it
does have no information about targets (there are no `-t` arguments set) at that point.

This exception leads to the limitation that the `helm-chart.yaml` MUST be valid yaml even in case it is not rendered
via the templating engine. This makes using control statements (if/for/...) impossible in this file. It also makes it
a requirement to use quotes around values that contain templates (e.g. the namespace in the above example).

`helm-values.yaml` is not subject to these limitations as it is only interpreted while deploying.
