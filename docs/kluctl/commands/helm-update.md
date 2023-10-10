<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "helm-update"
linkTitle: "helm-update"
weight: 10
description: >
    helm-update command
---
-->

## Command
<!-- BEGIN SECTION "helm-update" "Usage" false -->
Usage: kluctl helm-update [flags]

Recursively searches for 'helm-chart.yaml' files and checks for new available versions
Optionally performs the actual upgrade and/or add a commit to version control.

<!-- END SECTION -->

## Arguments
The following sets of arguments are available:
1. [project arguments](./common-arguments.md#project-arguments) (except `-a`)

In addition, the following arguments are available:
<!-- BEGIN SECTION "helm-update" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --commit                                      Create a git commit for every updated chart
      --helm-insecure-skip-tls-verify stringArray   Controls skipping of TLS verification. Must be in the form
                                                    --helm-insecure-skip-tls-verify=<host>/<path> or in the
                                                    deprecated form
                                                    --helm-insecure-skip-tls-verify=<credentialsId>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.
      --helm-key-file stringArray                   Specify client certificate to use for Helm Repository
                                                    authentication. Must be in the form
                                                    --helm-key-file=<host>/<path>=<filePath> or in the deprecated
                                                    form --helm-key-file=<credentialsId>:<filePath>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.
      --helm-password stringArray                   Specify password to use for Helm Repository authentication.
                                                    Must be in the form --helm-password=<host>/<path>=<password>
                                                    or in the deprecated form
                                                    --helm-password=<credentialsId>:<password>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.
      --helm-username stringArray                   Specify username to use for Helm Repository authentication.
                                                    Must be in the form --helm-username=<host>/<path>=<username>
                                                    or in the deprecated form
                                                    --helm-username=<credentialsId>:<username>, where
                                                    <credentialsId> must match the id specified in the helm-chart.yaml.
  -i, --interactive                                 Ask for every Helm Chart if it should be upgraded.
      --upgrade                                     Write new versions into helm-chart.yaml and perform helm-pull
                                                    afterwards

```
<!-- END SECTION -->