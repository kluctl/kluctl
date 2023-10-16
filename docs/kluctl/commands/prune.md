<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "prune"
linkTitle: "prune"
weight: 10
description: >
    prune command
---
-->

## Command
<!-- BEGIN SECTION "prune" "Usage" false -->
Usage: kluctl prune [flags]

Searches the target cluster for prunable objects and deletes them
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
<!-- BEGIN SECTION "prune" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --dry-run                     Performs all kubernetes API calls in dry-run mode.
      --no-obfuscate                Disable obfuscation of sensitive/secret data
  -o, --output-format stringArray   Specify output format and target file, in the format 'format=path'. Format can
                                    either be 'text' or 'yaml'. Can be specified multiple times. The actual format
                                    for yaml is currently not documented and subject to change.
      --render-output-dir string    Specifies the target directory to render the project into. If omitted, a
                                    temporary directory is used.
      --short-output                When using the 'text' output format (which is the default), only names of
                                    changes objects are shown instead of showing all changes.
  -y, --yes                         Suppresses 'Are you sure?' questions and proceeds as if you would answer 'yes'.

```
<!-- END SECTION -->

They have the same meaning as described in [deploy](./prune.md).
