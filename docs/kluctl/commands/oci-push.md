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
The push command creates a tarball from the given directory or the single file and uploads the
artifact to an OCI repository. The command can read the credentials from '~/.docker/config.json' but they can also be
passed with --creds. It can also login to a supported provider with the --provider flag.

<!-- END SECTION -->

## Arguments

The following arguments are available:
<!-- BEGIN SECTION "oci push" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --annotation stringArray    Set custom OCI annotations in the format '<key>=<value>'
      --creds string              credentials for OCI registry in the format <username>[:<password>] if --provider
                                  is generic
      --ignore-path stringArray   set paths to ignore in .gitignore format.
      --output string             the format in which the artifact digest should be printed, can be 'json' or 'yaml'
      --provider string           the OCI provider name, available options are: (generic, aws, azure, gcp)
                                  (default "generic")
      --timeout duration          Specify timeout for all operations, including loading of the project, all
                                  external api calls and waiting for readiness. (default 10m0s)
      --url string                Specifies the artifact URL. This argument is required.

```
<!-- END SECTION -->

