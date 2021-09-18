# Deployment projects

A deployment project defines all deployments, resources and configuration required to deploy our application and/or
base infrastructure. It consists of multiple yaml files interpreted by kluctl and other resources interpreted by
external tools (e.g. Helm or Kustomize).

## Basic structure

The following visualization shows the basic structure of a deployment project. The entry point of every deployment
project is the `deployment.yml` file, which then includes further sub-deployments and kustomize deployments. It also
provides some additional configuration required for multiple kluctl features to work as expected.

As can be seen, sub-deployments can include other sub-deployments, allowing you to structure the deployment project
as you need. You can for example use this to group persistency related deployments and non-persistent deployments.

Each level in this structure recursively adds [tags](./tags.md) to each deployed resources, allowing you to control
precisely what is deployed in the future.

Some visualized files/directories have links attached, follow them to get more information.

<pre>
-- project-dir/
   |-- <a href="#deploymentyml">deploymentyml</a>
   |-- .gitignore
   |-- kustomize-deployment1/
   |   |-- kustomization.yml
   |   `-- resource.yml
   |-- sub-deployment/
   |   |-- deployment.yml
   |   |-- kustomize-deployment2/
   |   |   |-- kustomization.yml
   |   |   |-- resource1.yml
   |   |   `-- ...
   |   |-- kustomize-deployment3/
   |   |   |-- kustomization.yml
   |   |   |-- resource1.yml
   |   |   |-- resource2.yml.jinja2
   |   |   |-- patch1.yml
   |   |   `-- ...
   |   |-- <a href="./helm-integration.md">kustomize-with-helm-deployment/</a>
   |   |   |-- charts/
   |   |   |   `-- ...
   |   |   |-- kustomization.yml
   |   |   |-- helm-chart.yml
   |   |   `-- helm-values.yml
   |   `-- subsub-deployment/
   |       |-- deployment.yml
   |       |-- ... kustomize deployments
   |       `-- ... subsubsub deployments
   `-- sub-deployment/
       `-- ...
</pre>

## Jinja2 Templating

Every file that is below the deployment project directory is also considered a [Jinja2](https://palletsprojects.com/p/jinja/)
template. This means, that it is rendered by Jinja2 before it is interpreted by kluctl or sent to kubernetes.

The only implicit exception is when Helm charts got pulled into the deployment project, as conflicts between Helms
templates and kluctl's templating would otherwise be guaranteed.

In case you need to exclude one or more files from Jinja2 rendering/templating, use the property [templateExcludes](#templateexcludes)
inside `deployment.yml`.

Documentation on available variables, methods and filters is available in [jinja2-templating](./jinja2-templating.md)

## Conatainer image versions

Please read [images](./images.md) about dynamic image versions.

## deployment.yml

The `deployment.yml` file is the entrypoint for the deployment project. Included sub-deployments also provide a
`deployment.yml` file with the same structure as the initial one.

An example `deployment.yml` looks like this:
```yaml
sealedSecrets:
  outputPattern: "{{ cluster.name }}/{{ args.environment }}"

kustomizeDirs:
- path: nginx
- path: my-app

includes:
- path: monitoring

commonLabels:
  my.prefix/environment: "{{ args.environment }}"
  my.prefix/deployment-project: k8s-deployment-airsea

deleteByLabels:
  my.prefix/environment: "{{ args.environment }}"
  my.prefix/deployment-project: k8s-deployment-airsea

args:
- name: environment
```

The following sub-chapters describe the available properties/fields in the `deployment.yml`

### sealedSecrets
`sealedSecrets` configures how sealed secrets are stored while sealing and located while rendering.
See [Sealed Secrets](./sealed-secrets.md#outputpattern-and-location-of-stored-sealed-secrets) for details.

### kustomizeDirs

`kustomizeDirs` is a list of [kustomize](https://kustomize.io/) deployments to be included in the deployment. The
kustomize deployment must be located in a directory that is relative to the location of the `deployment.yml`.

Please see [Kustomize integration](./kustomize-integration.md) for more details.

#### path
The relative path of the kustomize deployment. It must contain a valid `kustomization.yml`.

#### tags
A list of tags the kustomize deployment should have. See [tags](./tags.md) for more details.

#### barrier
Causes kluctl to wait until the current and all previous kustomize deployments have been applied. This is useful when
upcoming deployments need the current or previous deployments to be finished beforehand.

#### alwaysDeploy
Forces a kustomize Deployment to be included everytime, ignoring inclusion/exclusion sets from the command line.
See [Deploying with tag inclusion/exclusion](./tags.md#deploying-with-tag-inclusionexclusion) for details.

#### skipDeleteIfTags
Forces exclusion of a kustomize deployment whenever inclusion/exclusion tags are specified via command line.
See [Deleting with tag inclusion/exclusion](./tags.md#deleting-with-tag-inclusionexclusion) for details.

### vars
A list of additional sets of variables to be added to the Jinja2 context.

There are currently two types of entries possible:
1. If `values` is present, it directly contains the new variables
1. If `file` is present, it points to a relative yaml file that contains the variables.

Example:
```yaml
vars:
- file: vars1.yml
- values:
    var1: value1
    var2:
      var3: value3
```

See [jinja2-templating](./jinja2-templating.md) for more details.

### includes
A list of sub-deployment projects to include. These are deployment-projects as well and inherit many of the
properties of the parent deployment project.

#### path
The relative path of the sub-deployment project. It must contain a valid `deployment.yml`.

#### tags
A list of tags the include and all of its sub-includes and kustomize deployments should have.
See [tags](./tags.md) for more details.

#### barrier
Same as `barrier` in `kustomizeDirs`.

### commonLabels
A dictionary of [labels](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) and values to be
added to all resources deployed by any of the kustomize deployments in this deployment project.

This feature is mainly meant to make it possible to identify all objects in a kubernetes cluster that were once deployed
through a specific deployment project.

Consider the following example `deployment.yml`:
```yaml
kustomizeDirs:
  - path: nginx

includes:
  - path: sub-deployment1

commonLabels:
  my.prefix/deployment-name: my-deployment-project-name
  my.prefix/environment-name: {{ args.environment }}
  my.prefix/label-1: value-1
  my.prefix/label-2: value-2

# PLEASE read through the documentation for this field to understand why it should match commonLabels!
deleteByLabels:
  my.prefix/deployment-name: my-deployment-project-name
  my.prefix/environment-name: {{ args.environment }}
  my.prefix/label-1: value-1
  my.prefix/label-2: value-2
```

Every resource deployed by the kustomize deployment `nginx` will now get the two provided labels attached. All included
sub-deployment projects (e.g. `sub-deployment1`) will also recursively inherit these labels and pass them to further
down.

In case an included sub-deployment project also contains `commonLabels`, both dictionaries of common labels are merged
inside the included sub-deployment project. In case of conflicts, the included common labels override the inherited.

Please note that these `commonLabels` are not related to `commonLabels` supported in `kustomization.yml` files. It was
decided to not rely on this feature but instead attach labels manually to resources right before sending them to
kubernetes. This is due to an [implementation detail](https://github.com/kubernetes-sigs/kustomize/issues/1009) in
kustomize which causes `commonLabels` to also be applied to label selectors, which makes otherwise editable resources
read-only when it comes to `commonLabels`.

### deleteByLabels
A dictionary of labels used to filter resources when performing `kluctl delete` or `kluctl purge` operations.
It should usually match `commonLabels`, but can also omit parts of `commonLabels` (DANGEROUS!!!). It should however
never add labels that are not present in `commonLabels`.

Having `deleteByLabels` correct is crucial, as it might other lead to unrelated matches when searching for objects
to delete. This might then cause deletion of object that are NOT related to your deployment project.

### overrideNamespace
A string that is used as the default namespace for all kustomize deployments which don't have a `namespace` set in their
`kustomization.yml`.

### tags
A list of common tags which are applied to all kustomize deployments and sub-deployment includes.

See [tags](./tags.md) for more details.

### args
A list of arguments that can or must be passed to most kluctl operations. Each of these arguments is then available
in Jinja2 templating via the global `args` object. Only the root `deployment.yml` can contain such argument definitions.

An example looks like this:
```yaml
kustomizeDirs:
  - name: nginx

args:
  - name: environment
  - name: enable_debug
    default: "false"
```

These arguments can then be used in templating, e.g. by using `{{ args.environment }}`.

When calling kluctl, most of the commands will then require you to specify at least `-a environment=xxx` and optionally
`-a enable_debug=true`

The following sub chapters describe the fields for argument entries.

#### name
The name of the argument.

#### default
If specified, the argument becomes optional and will use the given value as default when not specified.

### templateExcludes
A list of file patterns to exclude from Jinja2 rendering/templating. This is important if you encounter issues with
resources containing sequences of characters that are misinterpreted by Jinja2. An example would be a configuration file
that includes Go templates, which will in most cases make Jinja2 templating fail.

### ignoreForDiff

A list of objects and fields to ignore while performing diffs. Consider the following example:

```yaml
kustomizeDirs:
  - ...

ignoreForDiff:
  - group: apps
    kind: Deployment
    namespace: my-namespace
    name: my-deployment
    fieldPath: spec.replicas
```

This will remove the `spec.replicas` field from every resource that matches the object.
`group`, `kind`, `namespace` and `name` can be omitted, which results in all objects matching. `fieldPath` must be a
valid [JSON Path](https://goessner.net/articles/JsonPath/). `fieldPath` may also be a list of JSON paths.

The JSON Path implementation used in kluctl has extended support for wildcards in field
names, allowing you to also specify paths like `metadata.labels.my-prefix-*`.

# Order of deployment
Deployments are done in parallel, meaning that there are usually no order guarantees. The only way to somehow control
order, is by placing [barriers](#barrier) between kustomize deployments. You should however not overuse barriers, as
they negatively impact the speed of kluctl.
