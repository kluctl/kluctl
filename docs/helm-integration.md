# Helm Integration

kluctl offers a simple-to-use Helm integration, which allows you to reuse many common third-party Helm Charts.

The integration is split into 2 parts/steps/layers. The first is the management and pulling of the Helm Charts, while
the second part handles configuration/customization and deployment of the chart.

## How it works

Helm charts are not directly deployed via Helm. Instead, kluctl renders the Helm Chart into a single file and then
hands over the rendered yaml to [kustomize](https://kustomize.io/). Rendering is done in combination with a provided
`helm-values.yml`, which contains the necessary values to configure the Helm Chart.

The resulting rendered yaml is then referred by your `kustomization.yml`, from which point on the
[kustomize integration](kustomize-integration.md) takes over. This means, that you can perform all desired
customization (patches, namespace override, ...) as if you provided your own resources via yaml files.

### Helm hooks

[Helm Hooks](https://helm.sh/docs/topics/charts_hooks/) are implemented by mapping them to [kluctl hooks](./hooks.md),
based on the following mapping table:

| Helm hook     | kluctl hook         |
|---------------|---------------------|
| pre-install   | pre-deploy-initial  |
| post-install  | post-deploy-initial |
| pre-delete    | Not supported       |
| post-delete   | Not supported       |
| pre-upgrade   | pre-deploy          |
| post-upgrade  | post-deploy         |
| pre-rollback  | Not supported       |
| post-rollback | Not supported       |
| test          | Not supported       |

Please note that this is a best effort approach and not 100% compatible to how Helm would run hooks. 

## helm-chart.yml

The `helm-chart.yml` defines where to get the chart from, which version should be pulled, the rendered output file name,
and a few more Helm options. After this file is added to your project, you need to invoke the `helm-pull` command
to pull the Helm Chart into your local project. It is advised to put the pulled Helm Chart into version control, so
that deployments will always be based on the exact same Chart (Helm does not guarantee this when pulling).

Example `helm-chart.yml`:

```yaml
helmChart:
  repo: https://charts.bitnami.com/bitnami
  chartName: redis
  chartVersion: 12.1.1
  skipUpdate: false
  releaseName: redis-cache
  namespace: "{{ my.jinja2.var }}"
  output: deploy.yml
```

When running the `helm-pull` command, it will recursively search for all `helm-chart.yml` files and then pull the
chart from the specified repository with the specified version. The pull chart will then be located in the sub-directory
`charts` below the same directory as the `helm-chart.yml`

The same filename that was specified in `output` must then be referred in a `kustomization.yml` as a normal local
resource.

`helmChart` inside `helm-chart.yml` supports the following fields:

### repo
The url to the Helm repository where the Helm Chart is located. You can use hub.helm.sh to search for repositories and
charts and then use the repos found there.

### chartName
The name of the chart that can be found in the repository.

### chartVersion
The version of the chart. 

### skipUpdate
Skip this Helm Chart when the [helm-update](./commands.md#helm-update) command is called. If omitted, defaults to `false`.

### releaseName
The name of the Helm Release.

### namespace
The namespace that this Helm Chart is going to be deployed to. Please note that this should match the namespace
that you're actually deploying the kustomize deployment to. This means, that either `namespace` in `kustomization.yml`
or `overrideNamespace` in `deployment.yml` should match the namespace given here. The namespace should also be existing
already at the point in time when the kustomize deployment is deployed.

### output
This is the file name into which the Helm Chart is rendered into. Your `kustomization.yml` should include this same
file. The file should not be existing in your project, as it is created on-the-fly while deploying.

### skipCRDs
If set to `true`, kluctl will pass `--skip-crds` to Helm when rendering the deployment. If set to `false` (which is
the default), kluctl will pass `--include-crds` to Helm.

## helm-values.yml
This file should be present when you need to pass custom Helm Value to Helm while rendering the deployment. Please
read the documentation of the used Helm Charts for details on what is supported.

## Updates to helm-charts
In case a Helm Chart needs to be updated, you can either do this manually by replacing the [chartVersion](#chartversion)
value in `helm-chart.yml` and the calling the [helm-pull](./commands.md#helm-pull) command or by simply invoking
[helm-update](./commands.md#helm-update) with `--upgrade` and/or `--commit` being set.

## Jinja2 Templating

Both `helm-chart.yml` and `helm-values.yml` are rendered by the [Jinaj2 templating](./jinja2-templating.md) before they
are actually used. This means, that you can use all available Jinja2 variables at that point, which can for example be
seen in the above `helm-chart.yml` example for the namespace.

There are however one exception that leads to a small limitation. When `helm-pull` reads the `helm-chart.yml`, it does
NOT render the file via Jinja2. This is because it can not know how to properly render Jinja2 as it does have no
information about the targeted environment (there are no `-a` arguments set) at that point.

This exception leads to the limitation that the `helm-chart.yml` MUST be valid yaml even in case it is not rendered
via Jinja2. This makes using control statements (if/for/...) impossible in this file. It also makes it a requirement
to use quotes around values that contain templates (e.g. the namespace in the above example).

`helm-values.yml` is not subject to these limitations as it is only interpreted while deploying.
