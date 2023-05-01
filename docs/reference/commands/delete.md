<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "delete"
linkTitle: "delete"
weight: 10
description: >
    delete command
---
-->

## Command
<!-- BEGIN SECTION "delete" "Usage" false -->
Usage: kluctl delete [flags]

Delete a target (or parts of it) from the corresponding cluster
Objects are located based on the target discriminator.

WARNING: This command will also delete objects which are not part of your deployment
project (anymore). It really only decides based on the discriminator and does NOT
take the local target/state into account!

<!-- END SECTION -->

## Arguments
The following sets of arguments are available:
1. [project arguments](./common-arguments.md#project-arguments)
1. [image arguments](./common-arguments.md#image-arguments)
1. [inclusion/exclusion arguments](./common-arguments.md#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "delete" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --discriminator string                        Override the discriminator used to find objects for deletion.
      --dry-run                                     Performs all kubernetes API calls in dry-run mode.
      --helm-insecure-skip-tls-verify stringArray   Controls skipping of TLS verification. Must be in the form
                                                    --helm-insecure-skip-tls-verify=<credentialsId>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.
      --helm-key-file stringArray                   Specify client certificate to use for Helm Repository
                                                    authentication. Must be in the form
                                                    --helm-key-file=<credentialsId>:<path>, where <credentialsId>
                                                    must match the id specified in the helm-chart.yaml.
      --helm-password stringArray                   Specify password to use for Helm Repository authentication.
                                                    Must be in the form
                                                    --helm-password=<credentialsId>:<password>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.
      --helm-username stringArray                   Specify username to use for Helm Repository authentication.
                                                    Must be in the form
                                                    --helm-username=<credentialsId>:<username>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.
      --no-obfuscate                                Disable obfuscation of sensitive/secret data
  -o, --output-format stringArray                   Specify output format and target file, in the format
                                                    'format=path'. Format can either be 'text' or 'yaml'. Can be
                                                    specified multiple times. The actual format for yaml is
                                                    currently not documented and subject to change.
      --render-output-dir string                    Specifies the target directory to render the project into. If
                                                    omitted, a temporary directory is used.
      --short-output                                When using the 'text' output format (which is the default),
                                                    only names of changes objects are shown instead of showing all
                                                    changes.
  -y, --yes                                         Suppresses 'Are you sure?' questions and proceeds as if you
                                                    would answer 'yes'.

```
<!-- END SECTION -->

They have the same meaning as described in [deploy](./deploy.md).
