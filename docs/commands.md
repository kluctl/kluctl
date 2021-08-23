# Command line interface

kluctl offers a unified command line interface that allows to standardize all your deployments. Every project,
no matter how different it is from other projects, is managed the same way.

You can always call `kluctl --help` or `kluctl <command> --help` for a help prompt.

# Common command arguments
A few sets of arguments are common between multiple commands. These arguments are still part of the command itself and
must be placed *after* the command name.

The following sets of common arguments are available:

## project arguments
<!-- BEGIN SECTION "deploy" "Project arguments" true -->
```
  Project arguments:              Define where and how to load the kluctl project and its components from.
    -p, --project-url TEXT        Git url of the kluctl project. If not specified, the current directory will be used
                                  instead of a remote Git project
    -b, --project-ref TEXT        Git ref of the kluctl project. Only used when --project-url was given.
    -c, --project-config FILE     Location of the .kluctl.yml config file. Defaults to $PROJECT/.kluctl.yml
    --local-clusters DIRECTORY    Local clusters directory. Overrides the project from .kluctl.yml
    --local-deployment DIRECTORY  Local deployment directory. Overrides the project from .kluctl.yml
    --local-sealed-secrets DIRECTORY
                                  Local sealed-secrets directory. Overrides the project from .kluctl.yml
    --from-archive PATH           Load project (.kluctl.yml, cluster, ...) from archive. Given path can either be an
                                  archive file or a directory with the extracted contents.
    --deployment-name TEXT        Name of the kluctl deployment. Used when resolving sealed-secrets. Defaults to the
                                  base name of --local-deployment/--project-url
    --cluster TEXT                Specify/Override cluster
    -a, --arg TEXT                Template argument in the form name=value
    -t, --target TEXT             Target name to run command for. Target must exist in .kluctl.yml.
```
<!-- END SECTION -->

These arguments control where and how to load the kluctl project and deployment project.

## image arguments
<!-- BEGIN SECTION "deploy" "Image arguments" true -->
```
  Image arguments: 
    -F, --fixed-image TEXT        Pin an image to a given version. Expects '--fixed-
                                  image=image<:namespace:deployment:container>=result'
    --fixed-images-file FILE      Use .yml file to pin image versions. See output of list-images sub-command or read
                                  the documentation for details about the output format
    -u, --update-images           This causes kluctl to prefer the latest image found in registries, based on the
                                  `latest_image` filters provided to 'images.get_image(...)' calls. Use this flag if
                                  you want to update to the latest versions/tags of all images. '-u' takes precedence
                                  over `--fixed-image/--fixed-images-file`, meaning that the latest images are used
                                  even if an older image is given via fixed images.
```
<!-- END SECTION -->

These arguments control image versions requested by `images.get_image(...)` [calls](./images.md#imagesget_image).

## Inclusion/Exclusion arguments
<!-- BEGIN SECTION "deploy" "Inclusion/Exclusion arguments" true -->
```
  Inclusion/Exclusion arguments: 
                                  Control inclusion/exclusion.
    -I, --include-tag TEXT        Include deployments with given tag.
    -E, --exclude-tag TEXT        Exclude deployments with given tag. Exclusion has precedence over inclusion, meaning
                                  that explicitly excluded deployments will always be excluded even if an inclusion
                                  rule would match the same deployment.
    --include-kustomize-dir TEXT  Include kustomize dir. The path must be relative to the root deployment project.
    --exclude-kustomize-dir TEXT  Exclude kustomize dir. The path must be relative to the root deployment project.
                                  Exclusion has precedence over inclusion, same as in --exclude-tag
```
<!-- END SECTION -->

These arguments control inclusion/exclusion based on tags and kustomize depoyment pathes.

# Commands
The following commands are available:

## bootstrap
<!-- BEGIN SECTION "bootstrap" "Usage" false -->
Usage: kluctl bootstrap [OPTIONS]

  Bootstrap a target cluster.

  This will install the sealed-secrets operator into the specified cluster if not already installed.

  Either --target or --cluster must be specified.

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "bootstrap" "Misc arguments" true -->
```
  Misc arguments: 
    -y, --yes                     Supresses 'Are you sure?' questions and proceeds as if you would answer 'yes'.
    --dry-run                     Performs all kubernetes API calls in dry-run mode.
    --parallel                    Run deployment in parallel instead of sequentially. See documentation for more
                                  details.
    --force-apply                 Force conflict resolution when applying. See documentation for details
    --replace-on-error            When patching an object fails, try to delete it and then retry. See documentation
                                  for more details.
    --abort-on-error              Abort deploying when an error occurs instead of trying the remaining deployments
    -o, --output TEXT             Specify output format and target file, in the format 'format=path'. Format can
                                  either be 'diff' or 'yaml'. Can be specified multiple times. The actual format for
                                  yaml is currently not documented and subject to change.
```
<!-- END SECTION -->

## deploy
<!-- BEGIN SECTION "deploy" "Usage" false -->
Usage: kluctl deploy [OPTIONS]

  Deploys a target to the corresponding cluster.

  This command will also output a diff between the initial state and the state after deployment. The format of this
  diff is the same as for the `diff` command. It will also output a list of purgable objects (without actually
  deleting them).

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "deploy" "Misc arguments" true -->
```
  Misc arguments: 
    -y, --yes                     Supresses 'Are you sure?' questions and proceeds as if you would answer 'yes'.
    --dry-run                     Performs all kubernetes API calls in dry-run mode.
    --parallel                    Run deployment in parallel instead of sequentially. See documentation for more
                                  details.
    --force-apply                 Force conflict resolution when applying. See documentation for details
    --replace-on-error            When patching an object fails, try to delete it and then retry. See documentation
                                  for more details.
    --abort-on-error              Abort deploying when an error occurs instead of trying the remaining deployments
    -o, --output TEXT             Specify output format and target file, in the format 'format=path'. Format can
                                  either be 'diff' or 'yaml'. Can be specified multiple times. The actual format for
                                  yaml is currently not documented and subject to change.
    --full-diff-after-deploy TEXT
                                  Perform a full diff (no inclusion/exclusion) directly after the deployent has
                                  finished. The argument has the same meaning as in `-o`
```
<!-- END SECTION -->

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
then overtaken by other managers (e.g. by operators). Always use this option with caution and perform a dry-run
before to ensure nothing unexpected gets overwritten.

### --replace-on-error
In some situations, updating Kubernetes objects is not possible, for example when modified fields are read-only. Jobs
are a good example where this might be the case. In such cases, you can use `--replace-on-error` to instruct kluctl to
retry an update by deleting and then recreating the object.

Please note that this is a potentially risky operation, especially when an object carries some kind of important state.

### --abort-on-error
kluctl does not abort a command when an individual object fails can not be updated. It collects all errors and warnings
and outputs them instead. This option modifies the behaviour to immediately abort the command.

## diff
<!-- BEGIN SECTION "diff" "Usage" false -->
Usage: kluctl diff [OPTIONS]

  Perform a diff between the locally rendered target and the already deployed target.

  The output is by default in human readable form (a table combined with unified diffs). The output can also be
  changed to output yaml file. Please note however that the format is currently not documented and prone to changes.

  After the diff is performed, the command will also search for purgable objects and list them.

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "diff" "Misc arguments" true -->
```
  Misc arguments: 
    --force-apply                 Force conflict resolution when applying. See documentation for details
    --replace-on-error            When patching an object fails, try to delete it and then retry. See documentation
                                  for more details.
    --ignore-tags                 Ignores changes in tags when diffing
    --ignore-labels               Ignores changes in labels when diffing
    --ignore-annotations          Ignores changes in annotations when diffing
    --ignore-order                Ignores changes in order when diffing
    -o, --output TEXT             Specify output format and target file, in the format 'format=path'. Format can
                                  either be 'diff' or 'yaml'. Can be specified multiple times. The actual format for
                                  yaml is currently not documented and subject to change.
```
<!-- END SECTION -->

`--force-apply` and `--replace-on-error` have the same meaning as in [deploy](#deploy).

## delete
<!-- BEGIN SECTION "delete" "Usage" false -->
Usage: kluctl delete [OPTIONS]

  Delete the a target (or parts of it) from the corresponding cluster.

  Objects are located based on `deleteByLabels`, configured in `deployment.yml`

  WARNING: This command will also delete objects which are not part of your deployment project (anymore). It really
  only decides based on the `deleteByLabel` labels and does NOT take the local target/state into account!

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "delete" "Misc arguments" true -->
```
  Misc arguments: 
    -y, --yes                     Supresses 'Are you sure?' questions and proceeds as if you would answer 'yes'.
    --dry-run                     Performs all kubernetes API calls in dry-run mode.
```
<!-- END SECTION -->

They have the same meaning as described in [deploy](#deploy).

## purge
<!-- BEGIN SECTION "purge" "Usage" false -->
Usage: kluctl purge [OPTIONS]

  Searches the target cluster for purgable objects and deletes them.

  Searching works by:

    1. Search the cluster for all objects match `deleteByLabels`, as configured in `deployment.yml`
    2. Render the local target and list all objects.
    3. Remove all objects from the list of 1. that are part of the list in 2.

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "purge" "Misc arguments" true -->
```
  Misc arguments: 
    -y, --yes                     Supresses 'Are you sure?' questions and proceeds as if you would answer 'yes'.
    --dry-run                     Performs all kubernetes API calls in dry-run mode.
```
<!-- END SECTION -->

They have the same meaning as described in [deploy](#deploy).

## list-images
<!-- BEGIN SECTION "list-images" "Usage" false -->
Usage: kluctl list-images [OPTIONS]

  Renders the target and outputs all images used via `images.get_image(...)`

  The result is a compatible with yaml files expected by --fixed-images-file.

  If fixed images (`-f/--fixed-image`) are provided, these are also taken into account, as described in for the deploy
  command.

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "list-images" "Misc arguments" true -->
```
  Misc arguments: 
    -o, --output TEXT             Specify output target file. Can be specified multiple times
    --no-kubernetes               Don't check kubernetes for current image versions
    --no-registries               Don't check registries for new image versions
    --simple                      Output a simplified version of the images list
```
<!-- END SECTION -->

## render
<!-- BEGIN SECTION "render" "Usage" false -->
Usage: kluctl render [OPTIONS]

  Renders all resources and configuration files and stores the result in either a temporary directory or a specified
  directory.

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "render" "Misc arguments" true -->
```
  Misc arguments: 
    --output-dir DIRECTORY        Specified the target directory to render the project into. If omitted, a random
                                  temporary directory is created and printed to stdout.
    --output-images FILE          Also output images list to given FILE. This output the same result as from the list-
                                  images command.
    --offline                     Go offline, meaning that kubernetes and registries are not asked for image versions
```
<!-- END SECTION -->

## validate
<!-- BEGIN SECTION "validate" "Usage" false -->
Usage: kluctl validate [OPTIONS]

  Validates the already deployed deployment.

  This means that all objects are retrieved from the cluster and checked for readiness.

  TODO: This needs to be better documented!

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "validate" "Misc arguments" true -->
```
  Misc arguments: 
    -o, --output TEXT             Specify output target file. Can be specified multiple times
    --wait TEXT                   Wait for the given amount of time until the deployment validates
    --sleep TEXT                  Sleep duration between validation attempts
    --warnings-as-errors          Consider warnings as failures
```
<!-- END SECTION -->

## seal
<!-- BEGIN SECTION "seal" "Usage" false -->
Usage: kluctl seal [OPTIONS]

  Seal secrets based on target's sealingConfig.

  Loads all secrets from the specified secrets sets from the target's sealingConfig and then renders the target,
  including all files with the `.sealme` extension. Then runs kubeseal on each `.sealme` file and stores secrets in
  the directory specified by `--local-sealed-secrets`, using the outputPattern from your deployment project.

  If no `--target` is specified, sealing is performed for all targets.

<!-- END SECTION -->

See [sealed-secrets](./sealed-secrets.md) for more details.

The following sets of arguments are available:
1. [project arguments](#project-arguments) (except `-a`)

In addition, the following arguments are available:
<!-- BEGIN SECTION "seal" "Misc arguments" true -->
```
  Misc arguments: 
    --secrets-dir DIRECTORY       Specifies where to find unencrypted secret files. The given directory is NOT meant
                                  to be part of your source repository! The given path only matters for secrets of
                                  type 'path'. Defaults to the current working directory.
    --force-reseal                Lets kluctl ignore secret hashes found in already sealed secrets and thus forces
                                  resealing of those.
```
<!-- END SECTION -->

## helm-pull
<!-- BEGIN SECTION "helm-pull" "Usage" false -->
Usage: kluctl helm-pull [OPTIONS]

  Recursively searches for `helm-chart.yml` files and pulls the specified Helm charts.

  The Helm charts are stored under the sub-directory `charts/<chart-name>` next to the `helm-chart.yml`. These Helm
  charts are meant to be added to version control so that pulling is only needed when really required (e.g. when the
  chart version changes).

<!-- END SECTION -->

See [helm-integration](./helm-integration.md) for more details.

The following sets of arguments are available:
1. [project arguments](#project-arguments) (except `-a`)

## helm-update
<!-- BEGIN SECTION "helm-update" "Usage" false -->
Usage: kluctl helm-update [OPTIONS]

  Recursively searches for `helm-chart.yml` files and checks for new available versions.

  Optionally performs the actual upgrade and/or add a commit to version control.

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments) (except `-a`)

In addition, the following arguments are available:
<!-- BEGIN SECTION "helm-update" "Misc arguments" true -->
```
  Misc arguments: 
    --upgrade                     Write new versions into helm-chart.yml and perform helm-pull afterwards
    --commit                      Create a git commit for every updated chart
```
<!-- END SECTION -->

## list-image-tags
<!-- BEGIN SECTION "list-image-tags" "Usage" false -->
Usage: kluctl list-image-tags [OPTIONS]

  Queries the tags for the given images.

<!-- END SECTION -->

The following arguments are available:
<!-- BEGIN SECTION "list-image-tags" "Misc arguments" true -->
```
  Misc arguments: 
    -o, --output TEXT  Specify output target file. Can be specified multiple times
    --image TEXT       Name of the image. Can be specified multiple times  [required]
```
<!-- END SECTION -->

## list-targets
<!-- BEGIN SECTION "list-targets" "Usage" false -->
Usage: kluctl list-targets [OPTIONS]

  Outputs a yaml list with all target, including dynamic targets

<!-- END SECTION -->

The following arguments are available:
<!-- BEGIN SECTION "list-targets" "Misc arguments" true -->
```
  Misc arguments: 
    -o, --output TEXT             Specify output target file. Can be specified multiple times
```
<!-- END SECTION -->

## archive
<!-- BEGIN SECTION "archive" "Usage" false -->
Usage: kluctl archive [OPTIONS]

  Write project and all related components into single tgz.

  This archive can then be used with `--from-archive`.

<!-- END SECTION -->

The following arguments are available:
<!-- BEGIN SECTION "archive" "Misc arguments" true -->
```
  Misc arguments: 
    --output PATH                 Path to .tgz to write project to.
    --reproducible                Make .tgz reproducible.
```
<!-- END SECTION -->
