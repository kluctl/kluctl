<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "deploy"
linkTitle: "deploy"
weight: 10
description: >
    deploy command
---
-->

## Command
<!-- BEGIN SECTION "deploy" "Usage" false -->
Usage: kluctl deploy [flags]

Deploys a target to the corresponding cluster
This command will also output a diff between the initial state and the state after
deployment. The format of this diff is the same as for the 'diff' command.
It will also output a list of prunable objects (without actually deleting them).

<!-- END SECTION -->

## Arguments
The following sets of arguments are available:
1. [project arguments](./common-arguments.md#project-arguments)
1. [image arguments](./common-arguments.md#image-arguments)
1. [inclusion/exclusion arguments](./common-arguments.md#inclusionexclusion-arguments)
1. [command results arguments](./common-arguments.md#command-results-arguments)
1. [helm arguments](./common-arguments.md#helm-arguments)
1. [registry arguments](./common-arguments.md#registry-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "deploy" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --abort-on-error               Abort deploying when an error occurs instead of trying the remaining deployments
      --dry-run                      Performs all kubernetes API calls in dry-run mode.
      --force-apply                  Force conflict resolution when applying. See documentation for details
      --force-replace-on-error       Same as --replace-on-error, but also try to delete and re-create objects. See
                                     documentation for more details.
      --no-obfuscate                 Disable obfuscation of sensitive/secret data
      --no-wait                      Don't wait for objects readiness.
  -o, --output-format stringArray    Specify output format and target file, in the format 'format=path'. Format
                                     can either be 'text' or 'yaml'. Can be specified multiple times. The actual
                                     format for yaml is currently not documented and subject to change.
      --prune                        Prune orphaned objects directly after deploying. See the help for the 'prune'
                                     sub-command for details.
      --readiness-timeout duration   Maximum time to wait for object readiness. The timeout is meant per-object.
                                     Timeouts are in the duration format (1s, 1m, 1h, ...). If not specified, a
                                     default timeout of 5m is used. (default 5m0s)
      --render-output-dir string     Specifies the target directory to render the project into. If omitted, a
                                     temporary directory is used.
      --replace-on-error             When patching an object fails, try to replace it. See documentation for more
                                     details.
      --short-output                 When using the 'text' output format (which is the default), only names of
                                     changes objects are shown instead of showing all changes.
  -y, --yes                          Suppresses 'Are you sure?' questions and proceeds as if you would answer 'yes'.

```
<!-- END SECTION -->

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
In some situations, patching Kubernetes objects might fail for different reasons. In such cases, you can try
`--replace-on-error` to instruct kluctl to retry with an update operation.

Please note that this will cause all fields to be overwritten, even if owned by other field managers.

### --force-replace-on-error
This flag will cause the same replacement attempt on failure as with `--replace-on-error`. In addition, it will fallback
to a delete+recreate operation in case the replace also fails.

Please note that this is a potentially risky operation, especially when an object carries some kind of important state.

### --abort-on-error
kluctl does not abort a command when an individual object fails can not be updated. It collects all errors and warnings
and outputs them instead. This option modifies the behaviour to immediately abort the command.
