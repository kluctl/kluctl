---
title: "helm-update"
linkTitle: "helm-update"
weight: 10
description: >
    helm-update command
---

## Command
<!-- BEGIN SECTION "helm-update" "Usage" false -->
Usage: kluctl helm-update [flags]

Recursively searches for 'helm-chart.yaml' files and checks for new available versions
Optionally performs the actual upgrade and/or add a commit to version control.

<!-- END SECTION -->

## Arguments
The following sets of arguments are available:
1. [project arguments]({{< ref "./common-arguments#project-arguments" >}}) (except `-a`)

In addition, the following arguments are available:
<!-- BEGIN SECTION "helm-update" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --commit                                 Create a git commit for every updated chart
      --insecure-skip-tls-verify stringArray   Controls skipping of TLS verification. Must be in the form
                                               --insecure-skip-tls-verify=<credentialsId>, where <credentialsId>
                                               must match the id specified in the helm-chart.yaml.
      --key-file stringArray                   Specify client certificate to use for Helm Repository
                                               authentication. Must be in the form
                                               --key-file=<credentialsId>:<path>, where <credentialsId> must match
                                               the id specified in the helm-chart.yaml.
      --password stringArray                   Specify password to use for Helm Repository authentication. Must be
                                               in the form --password=<credentialsId>:<password>, where
                                               <credentialsId> must match the id specified in the helm-chart.yaml.
      --upgrade                                Write new versions into helm-chart.yaml and perform helm-pull afterwards
      --username stringArray                   Specify username to use for Helm Repository authentication. Must be
                                               in the form --username=<credentialsId>:<username>, where
                                               <credentialsId> must match the id specified in the helm-chart.yaml.

```
<!-- END SECTION -->