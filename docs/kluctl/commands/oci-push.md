<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "oci push"
linkTitle: "oci push"
weight: 10
description: >
    oci push command
---
-->

## Command
<!-- BEGIN SECTION "oci push" "Usage" false -->
Usage: kluctl oci push [flags]

Push to an oci repository
The push command creates a tarball from the current project and uploads the
artifact to an OCI repository.

<!-- END SECTION -->

## Arguments
The following sets of arguments are available:
1. [registry arguments](./common-arguments.md#registry-arguments)

In addition, the following arguments are available:

<!-- BEGIN SECTION "oci push" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --annotation stringArray   Set custom OCI annotations in the format '<key>=<value>'
      --output string            the format in which the artifact digest should be printed, can be 'json' or 'yaml'
      --timeout duration         Specify timeout for all operations, including loading of the project, all
                                 external api calls and waiting for readiness. (default 10m0s)
      --url string               Specifies the artifact URL. This argument is required.

```
<!-- END SECTION -->

