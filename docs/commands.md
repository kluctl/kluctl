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
Project arguments:
  Define where and how to load the kluctl project and its components from.

  -p, --project-url=STRING              Git url of the kluctl project. If not specified, the current directory will be
                                        used instead of a remote Git project
  -b, --project-ref=STRING              Git ref of the kluctl project. Only used when --project-url was given.
  -c, --project-config=STRING           Location of the .kluctl.yml config file. Defaults to $PROJECT/.kluctl.yml
      --local-clusters=STRING           Local clusters directory. Overrides the project from .kluctl.yml
      --local-deployment=STRING         Local deployment directory. Overrides the project from .kluctl.yml
      --local-sealed-secrets=STRING     Local sealed-secrets directory. Overrides the project from .kluctl.yml
      --from-archive=STRING             Load project (.kluctl.yml, cluster, ...) from archive. Given path can either be
                                        an archive file or a directory with the extracted contents.
      --from-archive-metadata=STRING    Specify where to load metadata (targets, ...) from. If not specified, metadata
                                        is assumed to be part of the archive.
      --cluster=STRING                  Specify/Override cluster
  -t, --target=STRING                   Target name to run command for. Target must exist in .kluctl.yml.
  -a, --arg=ARG,...                     Template argument in the form name=value

```
<!-- END SECTION -->

These arguments control where and how to load the kluctl project and deployment project.

## image arguments
<!-- BEGIN SECTION "deploy" "Image arguments" true -->
```
Image arguments:
  Control fixed images and update behaviour.

  -F, --fixed-image=FIXED-IMAGE,...    Pin an image to a given version. Expects
                                       '--fixed-image=image<:namespace:deployment:container>=result'
      --fixed-images-file=STRING       Use .yml file to pin image versions. See output of list-images sub-command or
                                       read the documentation for details about the output format
  -u, --update-images                  This causes kluctl to prefer the latest image found in registries, based on the
                                       'latest_image' filters provided to 'images.get_image(...)' calls. Use this flag
                                       if you want to update to the latest versions/tags of all images. '-u' takes
                                       precedence over '--fixed-image/--fixed-images-file', meaning that the latest
                                       images are used even if an older image is given via fixed images.

```
<!-- END SECTION -->

These arguments control image versions requested by `images.get_image(...)` [calls](./images.md#imagesget_image).

## Inclusion/Exclusion arguments
<!-- BEGIN SECTION "deploy" "Inclusion/Exclusion arguments" true -->
```
Inclusion/Exclusion arguments:
  Control inclusion/exclusion.

  -I, --include-tag=INCLUDE-TAG,...    Include deployments with given tag.
  -E, --exclude-tag=EXCLUDE-TAG,...    Exclude deployments with given tag. Exclusion has precedence over inclusion,
                                       meaning that explicitly excluded deployments will always be excluded even if an
                                       inclusion rule would match the same deployment.
      --include-deployment-dir=INCLUDE-DEPLOYMENT-DIR,...
                                       Include deployment dir. The path must be relative to the root deployment project.
      --exclude-deployment-dir=EXCLUDE-DEPLOYMENT-DIR,...
                                       Exclude deployment dir. The path must be relative to the root deployment project.
                                       Exclusion has precedence over inclusion, same as in --exclude-tag

```
<!-- END SECTION -->

These arguments control inclusion/exclusion based on tags and kustomize depoyment pathes.

# Environment variables
All options/arguments accepted by kluctl can also be specified via environment variables. The name of the environment
variables always start with `KLUCTL_` and end witht the option/argument in uppercase and dashes replaced with
underscores. As an example, `--project=my-project` can also be specified with the environment variable
`KLUCTL_PROJECT=my-project`.

## Additional environment variables
A few additional environment variables are supported which do not belong to an option/argument. These are:

1. `KLUCTL_REGISTRY_<idx>_HOST`, `KLUCTL_REGISTRY_<idx>_USERNAME`, and so on. See [registries](./images.md#supported-image-registries-and-authentication) for details.
2. `KLUCTL_SSH_DISABLE_STRICT_HOST_KEY_CHECKING`. Disable ssh host key checking when accessing git repositories.
3. `KLUCTL_NO_THREADS`. Do not use multithreading while performing work. This is only useful for debugging purposes.
4. `KLUCTL_IGNORE_DEBUGGER`. Pretend that there is no debugger attached when automatically deciding if multi-threading should be enabled or not.


# Commands
The following commands are available:

## deploy
<!-- BEGIN SECTION "deploy" "Usage" false -->
Usage: kluctl deploy

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "deploy" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  -y, --yes                         Suppresses 'Are you sure?' questions and proceeds as if you would answer 'yes'.
      --dry-run                     Performs all kubernetes API calls in dry-run mode.
      --force-apply                 Force conflict resolution when applying. See documentation for details
      --replace-on-error            When patching an object fails, try to replace it. See documentation for more
                                    details.
      --force-replace-on-error      Same as --replace-on-error, but also try to delete and re-create objects. See
                                    documentation for more details.
      --abort-on-error              Abort deploying when an error occurs instead of trying the remaining deployments
      --hook-timeout=5m             Maximum time to wait for hook readiness. The timeout is meant per-hook. Timeouts are
                                    in the duration format (1s, 1m, 1h, ...). If not specified, a default timeout of 5m
                                    is used.
  -o, --output=OUTPUT,...           Specify output target file. Can be specified multiple times
      --render-output-dir=STRING    Specifies the target directory to render the project into. If omitted, a temporary
                                    directory is used.

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
Usage: kluctl diff

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "diff" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --force-apply                        Force conflict resolution when applying. See documentation for details
      --replace-on-error                   When patching an object fails, try to replace it. See documentation for more
                                           details.
      --force-replace-on-error             Same as --replace-on-error, but also try to delete and re-create objects. See
                                           documentation for more details.
      --ignore-tags                        Ignores changes in tags when diffing
      --ignore-labels                      Ignores changes in labels when diffing
      --ignore-annotations                 Ignores changes in annotations when diffing
  -o, --output-format=OUTPUT-FORMAT,...    Specify output format and target file, in the format 'format=path'. Format
                                           can either be 'text' or 'yaml'. Can be specified multiple times. The actual
                                           format for yaml is currently not documented and subject to change.
      --render-output-dir=STRING           Specifies the target directory to render the project into. If omitted, a
                                           temporary directory is used.

```
<!-- END SECTION -->

`--force-apply` and `--replace-on-error` have the same meaning as in [deploy](#deploy).

## delete
<!-- BEGIN SECTION "delete" "Usage" false -->
Usage: kluctl delete

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "delete" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  -y, --yes                                    Suppresses 'Are you sure?' questions and proceeds as if you would answer
                                               'yes'.
      --dry-run                                Performs all kubernetes API calls in dry-run mode.
  -o, --output-format=OUTPUT-FORMAT,...        Specify output format and target file, in the format 'format=path'.
                                               Format can either be 'text' or 'yaml'. Can be specified multiple times.
                                               The actual format for yaml is currently not documented and subject to
                                               change.
  -l, --delete-by-label=DELETE-BY-LABEL,...    Override the labels used to find objects for deletion.

```
<!-- END SECTION -->

They have the same meaning as described in [deploy](#deploy).

## prune
<!-- BEGIN SECTION "prune" "Usage" false -->
Usage: kluctl prune

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "prune" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  -y, --yes                                Suppresses 'Are you sure?' questions and proceeds as if you would answer
                                           'yes'.
      --dry-run                            Performs all kubernetes API calls in dry-run mode.
  -o, --output-format=OUTPUT-FORMAT,...    Specify output format and target file, in the format 'format=path'. Format
                                           can either be 'text' or 'yaml'. Can be specified multiple times. The actual
                                           format for yaml is currently not documented and subject to change.

```
<!-- END SECTION -->

They have the same meaning as described in [deploy](#deploy).

## list-images
<!-- BEGIN SECTION "list-images" "Usage" false -->
Usage: kluctl list-images

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "list-images" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  -o, --output=OUTPUT,...    Specify output target file. Can be specified multiple times
      --simple               Output a simplified version of the images list

```
<!-- END SECTION -->

## poke-images
<!-- BEGIN SECTION "poke-images" "Usage" false -->
Usage: kluctl poke-images

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "poke-images" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  -y, --yes                         Suppresses 'Are you sure?' questions and proceeds as if you would answer 'yes'.
      --dry-run                     Performs all kubernetes API calls in dry-run mode.
  -o, --output=OUTPUT,...           Specify output target file. Can be specified multiple times
      --render-output-dir=STRING    Specifies the target directory to render the project into. If omitted, a temporary
                                    directory is used.

```
<!-- END SECTION -->

## downscale
<!-- BEGIN SECTION "downscale" "Usage" false -->
Usage: kluctl downscale

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)
1. [inclusion/exclusion arguments](#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "downscale" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  -y, --yes                                Suppresses 'Are you sure?' questions and proceeds as if you would answer
                                           'yes'.
      --dry-run                            Performs all kubernetes API calls in dry-run mode.
  -o, --output-format=OUTPUT-FORMAT,...    Specify output format and target file, in the format 'format=path'. Format
                                           can either be 'text' or 'yaml'. Can be specified multiple times. The actual
                                           format for yaml is currently not documented and subject to change.
      --render-output-dir=STRING           Specifies the target directory to render the project into. If omitted, a
                                           temporary directory is used.

```
<!-- END SECTION -->

## render
<!-- BEGIN SECTION "render" "Usage" false -->
Usage: kluctl render

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "render" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  --render-output-dir=STRING    Specifies the target directory to render the project into. If omitted, a temporary
                                directory is used.

```
<!-- END SECTION -->

## validate
<!-- BEGIN SECTION "validate" "Usage" false -->
Usage: kluctl validate

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments)
1. [image arguments](#image-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "validate" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  -o, --output=OUTPUT,...           Specify output target file. Can be specified multiple times
      --render-output-dir=STRING    Specifies the target directory to render the project into. If omitted, a temporary
                                    directory is used.
      --wait=DURATION               Wait for the given amount of time until the deployment validates
      --sleep=5s                    Sleep duration between validation attempts
      --warnings-as-errors          Consider warnings as failures

```
<!-- END SECTION -->

## seal
<!-- BEGIN SECTION "seal" "Usage" false -->
Usage: kluctl seal

<!-- END SECTION -->

See [sealed-secrets](./sealed-secrets.md) for more details.

The following sets of arguments are available:
1. [project arguments](#project-arguments) (except `-a`)

In addition, the following arguments are available:
<!-- BEGIN SECTION "seal" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  --secrets-dir=STRING    Specifies where to find unencrypted secret files. The given directory is NOT meant to be part
                          of your source repository! The given path only matters for secrets of type 'path'. Defaults to
                          the current working directory.
  --force-reseal          Lets kluctl ignore secret hashes found in already sealed secrets and thus forces resealing of
                          those.

```
<!-- END SECTION -->

## helm-pull
<!-- BEGIN SECTION "helm-pull" "Usage" false -->
Usage: kluctl helm-pull

<!-- END SECTION -->

See [helm-integration](./helm-integration.md) for more details.

The following sets of arguments are available:
1. [project arguments](#project-arguments) (except `-a`)

## helm-update
<!-- BEGIN SECTION "helm-update" "Usage" false -->
Usage: kluctl helm-update

<!-- END SECTION -->

The following sets of arguments are available:
1. [project arguments](#project-arguments) (except `-a`)

In addition, the following arguments are available:
<!-- BEGIN SECTION "helm-update" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  --upgrade    Write new versions into helm-chart.yml and perform helm-pull afterwards
  --commit     Create a git commit for every updated chart

```
<!-- END SECTION -->

## list-targets
<!-- BEGIN SECTION "list-targets" "Usage" false -->
Usage: kluctl list-targets

<!-- END SECTION -->

The following arguments are available:
<!-- BEGIN SECTION "list-targets" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  -o, --output=OUTPUT,...    Specify output target file. Can be specified multiple times

```
<!-- END SECTION -->

## archive
<!-- BEGIN SECTION "archive" "Usage" false -->
Usage: kluctl archive

<!-- END SECTION -->

The following arguments are available:
<!-- BEGIN SECTION "archive" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  --output-archive=STRING     Path to .tgz to write project to.
  --output-metadata=STRING    Path to .yml to write metadata to. If not specified, metadata is written into the archive.

```
<!-- END SECTION -->
