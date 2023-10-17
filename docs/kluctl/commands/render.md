<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "render"
linkTitle: "render"
weight: 10
description: >
    render command
---
-->

## Command
<!-- BEGIN SECTION "render" "Usage" false -->
Usage: kluctl render [flags]

Renders all resources and configuration files
Renders all resources and configuration files and stores the result in either
a temporary directory or a specified directory.

<!-- END SECTION -->

## Arguments
The following sets of arguments are available:
1. [project arguments](./common-arguments.md#project-arguments)
1. [image arguments](./common-arguments.md#image-arguments)
1. [inclusion/exclusion arguments](./common-arguments.md#inclusionexclusion-arguments)
1. [helm arguments](./common-arguments.md#helm-arguments)
1. [registry arguments](./common-arguments.md#registry-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "render" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --kubernetes-version string   Specify the Kubernetes version that will be assumed. This will also override
                                    the kubeVersion used when rendering Helm Charts.
      --offline-kubernetes          Run command in offline mode, meaning that it will not try to connect the
                                    target cluster
      --print-all                   Write all rendered manifests to stdout
      --render-output-dir string    Specifies the target directory to render the project into. If omitted, a
                                    temporary directory is used.

```
<!-- END SECTION -->