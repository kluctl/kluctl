# Command line interface

kluctl offers a unified command line interface that allows to standardize all your deployments. Every project,
no matter how different is is from other projects, is managed the same way.

You can always call `kluctl --help` or `kluctl <command> --help` for a help prompt.

# Common cluster arguments
```
  Cluster specific options: 
    -C, --cluster-dir DIRECTORY  Directory that contains cluster
                                 configurations. Defaults to $PWD/clusters
    -N, --cluster-name TEXT      Name of the target cluster
```

Most commands expect you to at least provide the cluster name that you target the command for. This is specified via
`-N <name>`. The given name must match one of the cluster configurations found in the cluster configuration directory,
which is located at `$pwd/clusters` by default. You can also override the cluster configuration directory with
`-C <directory>`.

The cluster arguments must be placed *before* the command name.

# Common command arguments
A few sets of arguments are common between multiple commands. These arguments are still part of the command itself and
must be placed *after* the command name.

The following sets of common arguments are available:

## project arguments
```
  Project arguments: 
    -d, --deployment DIRECTORY    Deployment directory
    --deployment-name TEXT        Name of the deployment. Used when resolving
                                  sealed-secrets. Defaults to the base name of
                                  --deployment
    -a, --arg TEXT                Template argument
    --sealed-secrets-dir DIRECTORY
                                  Sealed secrets directory (default is
                                  $PWD/.sealed-secrets)
```

### -d, --deployment DIRECTORY
This argument specifies the deployment project location/directory. It is an optiona required argument which defaults
to the current working directory if omitted. See [deployments](./deployments.md) for details on how a deployment
project must be structured.

### -a, --arg TEXT
This argument adds an [argument](./deployments.md#args) to the context of the Jinja2 templating engine. The argument
must be in the form `name=value`, which will then make the value available via the variable `args.name`.

### --sealed-secrets-dir DIRECTORY
This argument specifies the base directory to look for and store sealed secrets. It is by default
`$pwd/.sealed-secrets`.

## image arguments
```
  Image arguments: 
    -F, --fixed-image TEXT        Pin an image to a given version. Expects '--
                                  fixed-image=image<:namespace:deployment:cont
                                  ainer>=result'
    --fixed-images-file FILE      Use .yml file to pin image versions. See
                                  output of list-images sub-command
    -u, --update-images           Update images to latest version found in the
                                  image registries
```

These arguments control the behaviour of the dynamic [image versions/tags](./images.md) handling.

### -F, --fixed-image TEXT
Pins an image to a specific image version/tag. Please see [images](./images.md#fixed-images-via-cli) for details.

### --fixed-images-file FILE
Please see [images](./images.md#fixed-images-via-a-yaml-file) for details.

### -u, --update-images
This causes kluctl to prefer the latest image found in registries, based on the `latest_image` filters provided to
`images.get_image(...)` calls. Use this flag if you want to update to the latest versions/tags of all images.

`-u` takes precedence over `--fixed-image/--fixed-images-file`, meaning that the latest images are used even if an older
image is given via fixed images.

## Inclusion/Exclusion arguments
```
  Inclusion/Exclusion arguments: 
    -I, --include-tag TEXT        Include deployments with given tag
    -E, --exclude-tag TEXT        Exclude deployments with given tag
    --include-kustomize-dir TEXT  Include kustomize dir
    --exclude-kustomize-dir TEXT  Exclude kustomize dir
```

Some commands support inclusion/exclusion arguments which allow controlling which kustomize deployments are actually
included in deployments.

Exclusion has precedence over inclusion, meaning that kustomize deployments will always be excluded when in the
exclusion set even if the inclusion set would include it.

Inclusion/Exclusion can be tag based or path based.
The `-I/--include-tag=<tag>` and `-E/--exclude-tag=<tag>` arguments specify tags to include/exclude.
`--include-kustomize-dir` and `--exclude-kustomize-dir` specify pathes to include/exclude. Pathes must be relative to
the deployment project root.

# Commands
The following commands are available:

## deploy
This command will deploy the deployment project to the specified cluster and then output the resulting diff between the
previous and the new state.

Please note that `deploy` does only list deleted objects, but not actually delete these.

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
```
  Misc arguments: 
    -y, --yes                     Answer yes on questions
    --dry-run                     Dry run
    --parallel                    Allow apply objects in parallel
    --force-apply                 Force conflict resolution when applying
    --replace-on-error            When patching an object fails, try to delete
                                  it and then retry
    --abort-on-error              Abort deploying when an error occurs instead
                                  of trying the remaining deployments
    -o, --output TEXT             Specify output format and target file, in
                                  the format 'format=path'. Format can either
                                  be 'diff' or 'yaml'. Can be specified
                                  multiple times
    --full-diff-after-deploy TEXT
                                  Perform a full diff (no inclusion/exclusion)
                                  directly after the deploy
```

### -y, --yes
Supresses "Are you sure?" questions and proceeds as if you'd answered `yes`.

### --dry-run
Performs the deployment in [dry-run](https://kubernetes.io/docs/reference/using-api/api-concepts/#dry-run) mode.

### --parallel
kluctl runs deployments sequentially and in-order by default. This options allows kluctl to perform all deployments
in parallel, which speeds up the deployment significantly.

Due to the nature of parallel deployments, no guarantees can't be made in regard to deployment order. This means for
example, that objects that are meant to be deployed into a namespace might be deployed before the namespace is deployed,
resulting in failure.

### --force-apply
kluctl implements deployments via [server-side apply](https://kubernetes.io/docs/reference/using-api/server-side-apply/)
and a custom automatic conflict resolution algorithm. This algurithm is an automatic implementation of the
"[Don't overwrite value, give up management claim](https://kubernetes.io/docs/reference/using-api/server-side-apply/#conflicts)"
method. It should work in most cases, but might still fail. In case of such failure, you can use `--force-apply` to
use the "Overwrite value, become sole manager" strategy instead.

Please note that this is a risky operation which might overwrite fields which were initially managed by kluctl but were
then overtaken by other managers (e.g. by operators). Always use this option with caution and perform a [dry-run](#--dry-run)
before to ensure nothing unexpected gets overwritten.

### --replace-on-error
In some situations, updating Kubernetes objects is not possible, for example when modified fields are read-only. Jobs
are a good example where this might be the case. In such cases, you can use `--replace-on-error` to instruct kluctl to
retry an update by deleting and then recreating the object.

Please note that this is a potentially risky operation, especially when an object carries some kind of important state.

### --abort-on-error
kluctl does not abort a command when an individual object fails can not be updated. It collects all errors and warnings
and outputs them instead. This option modifies the behaviour to immediately abort the command.

### -o, --output TEXT
Specifies where and how to output the resulting diff.

The output format is currently not documented and subject to changes.
Please use this with caution and expect breaking changes at any time. 

### --full-diff-after-deploy TEXT
Lets kluctl perform a full-diff (no inclusions/exclusions) after the deployment has finished. The argument has the same
meaning as in `-o`.

The output format is currently not documented and subject to changes.
Please use this with caution and expect breaking changes at any time. 

## diff
This command will prepare everything that is needed to perform a deployment and then run the deployment in dry-run mode.
Kubernetes will then return all objects after doing all the modifications (in dry-run mode) to the objects, which is
then compared to the objects that are already deployed. The diff of that is then displayed.

The command will also search for objects that are present on the kubernetes cluster but are missing locally. These
are the objects that got removed locally and should be [purged](#purge) at some point in time.

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
```
  Misc arguments: 
    --force-apply                 Force conflict resolution when applying
    --replace-on-error            When patching an object fails, try to delete
                                  it and then retry
    --ignore-tags                 Ignores changes in tags when diffing
    --ignore-labels               Ignores changes in labels when diffing
    --ignore-order                Ignores changes in order when diffing
    -o, --output TEXT             Specify output format and target file, in
                                  the format 'format=path'. Format can either
                                  be 'diff' or 'yaml'. Can be specified
                                  multiple times
```

`--force-apply`, `--replace-on-error` and `-o` have the same meaning as in [deploy](#deploy).

### --ignore-tags
Filters out changes in object tags (which are labels internally).

### --ignore-labels
filters out changes in labels.

### --ignore-order
Causes order in lists to be ignored when diffing. Useful for example when many environment variables have changed.

## delete
This command will delete the deployment (or parts of it, see [tags](./tags.md#deleting-with-tag-inclusionexclusion))
based on the [deleteByLabel](./deployments.md#deletebylabels) labels.

WARNING: This command will also delete objects which are not part of your deployment project (anymore). It really only
decides based on the `deleteByLabel` labels and does NOT take the local state into account!

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
```
  Misc arguments: 
    -y, --yes                     Answer yes on questions
    --dry-run                     Dry run
```

They have the same meaning as described in [deploy](#deploy).

## purge
This command will purge all objects that match the [deleteByLabel](./deployments.md#deletebylabels) labels and are not
found in the local deployment project anymore.

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
```
  Misc arguments: 
    -y, --yes                     Answer yes on questions
    --dry-run                     Dry run
```

They have the same meaning as described in [deploy](#deploy).

## list-images
This command prepares the deployment and instead of actually performing it, lists all images that were returned by
`images.get_image` while doing so. The result is a compatible with
[--fixed-images-file](#--fixed-images-file-file) yaml files.

If [fixed images](#-f---fixed-image-text) are provided, these are also taken into account, as described in for the
deploy command.

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
```
  Misc arguments: 
    -o, --output TEXT             Specify output target file. Can be specified
                                  multiple times
    --no-kubernetes               Don't check kubernetes for current image
                                  versions
    --no-registries               Don't check registries for new image
                                  versions
    --simple                      Return simple list
```

### -o, --output TEXT
Specifies where and how to output the resulting images list.

The output format is currently not documented and subject to changes.
Please use this with caution and expect breaking changes at any time. 

### --no-kubernetes
Disable querying kubernetes for currently deployed images.

### --no-registries
Disable querying image registries for latest images.

### --simple
Output a simplified version of the images list.

## render
Renders all resources and configuration files and stores the result in either a temporary directory or a specified
directory.

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)

In addition, the following arguments are available:
```
  Misc arguments: 
    --output-dir DIRECTORY        Write rendered output to specified directory
    --output-images FILE          Write images list to output file
    --offline                     Go offline, meaning that kubernetes and
                                  registries are not asked for image versions
```

### --output-dir DIRECTORY
Specified the target directory to render the project into. If omitted, a random temporary directory is created and
printed to stdout.

### --output-images FILE
Also output images list to given FILE. This output the same result as from the list-images command.

### --offline
Go offline, meaning that kubernetes and registries are not asked for image versions

## validate
Validates the already deployed deployment. This means that all objects are retrieved from the cluster and checked
for readiness.

TODO: This needs to be better documented!

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)

In addition, the following arguments are available:
```
  Misc arguments: 
    -o, --output TEXT             Specify output target file. Can be specified
                                  multiple times
    --wait TEXT                   Wait for the given amount of time until the
                                  deployment validates
    --sleep TEXT                  Sleep duration between validation attempts
    --warnings-as-errors          Consider warnings as failures
```

## seal
Recursively searches for `sealme-conf.yml` files and tries to seal each `.sealme` file that is recursively found below
the `sealme-conf.yml`. If a cluster is specified via `-N CLUSTER_NAME`, this is only tried for the specified cluster.
Otherwise, your kubeconfig is matched against all available [cluster configurations](./cluster-config.md) and sealing
is tried for each matching cluster.

See [sealed-secrets](./sealed-secrets.md) for more details.

The following sets of arguments are available:
1. [project arguments](#project-arguments) (except `-a`)

In addition, the following arguments are available:
```
  Misc arguments: 
    --secrets-dir DIRECTORY       Directory of unencrypted secret files
                                  (default is $PWD)
    --force-reseal                Force re-sealing even if a secret is
                                  determined as unchanged.
```

### --secrets-dir DIRECTORY
Specifies where to find unencrypted secret files. The given directory is NOT meant to be part of your source repository!
The given path only matters for secrets of type "path".

### --force-reseal
Lets kluctl ignore secret hashes found in already sealed secrets and thus forces resealing of those.

## helm-pull
Recursively searches for `helm-chart.yml` files and pulls the specified Helm charts. The Helm charts are stored under
the sub-directory `charts/<chart-name>` next to the `helm-chart.yml`.

These Helm charts are meant to be added to version control so that pulling is only needed when really required (e.g.
when the chart version changes).

See [helm-integration](./helm-integration.md) for more details.

The following sets of arguments are available:
1. [project arguments](#project-arguments) (except `-a`)

## helm-update
Recursively searches for `helm-chart.yml` files and checks for new available versions. Optionally performs the actual
upgrade and/or add a commit to version control.

The following sets of arguments are available:
1. [project arguments](#project-arguments) (except `-a`)

In addition, the following arguments are available:
```
  Misc arguments: 
    --upgrade                     Write new versions into helm-chart.yml and
                                  perform helm-pull afterwards
    --commit                      Create a git commit for every updated chart
```

### --upgrade
Not only check for an update, but also perform it (including a helm-pull call).

### --commit
Also commit the changes from `--upgrade` into version control.

## list-image-tags
Queries the tags for the given images.

The following arguments are available:
```
  Misc arguments: 
    -o, --output TEXT  Specify output target file. Can be specified multiple
                       times
    --image TEXT       Name of the image  [required]
```

### -o, --output TEXT
Specify output yaml file.

### --image TEXT
Specify which image to query tags for. Can be specified multiple times.
