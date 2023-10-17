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
1. [helm arguments](./common-arguments.md#helm-arguments)
1. [registry arguments](./common-arguments.md#registry-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "helm-update" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --commit        Create a git commit for every updated chart
  -i, --interactive   Ask for every Helm Chart if it should be upgraded.
      --upgrade       Write new versions into helm-chart.yaml and perform helm-pull afterwards

```
<!-- END SECTION -->