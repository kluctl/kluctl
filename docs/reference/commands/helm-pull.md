<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "helm-pull"
linkTitle: "helm-pull"
weight: 10
description: >
    helm-pull command
---
-->

## Command
<!-- BEGIN SECTION "helm-pull" "Usage" false -->
Usage: kluctl helm-pull [flags]

Recursively searches for 'helm-chart.yaml' files and pulls the specified Helm charts
The Helm charts are stored under the sub-directory 'charts/<chart-name>' next to the
'helm-chart.yaml'. These Helm charts are meant to be added to version control so that
pulling is only needed when really required (e.g. when the chart version changes).

<!-- END SECTION -->

See [helm-integration](../deployments/helm.md) for more details.

## Arguments
The following sets of arguments are available:
1. [project arguments](./common-arguments.md#project-arguments) (except `-a`)
