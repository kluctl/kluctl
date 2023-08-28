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

In addition, the following arguments are available:
<!-- BEGIN SECTION "render" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

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
      --kubernetes-version string                   Specify the Kubernetes version that will be assumed. This will
                                                    also override the kubeVersion used when rendering Helm Charts.
      --offline-kubernetes                          Run command in offline mode, meaning that it will not try to
                                                    connect the target cluster
      --print-all                                   Write all rendered manifests to stdout
      --render-output-dir string                    Specifies the target directory to render the project into. If
                                                    omitted, a temporary directory is used.

```
<!-- END SECTION -->