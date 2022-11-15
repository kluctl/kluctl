<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "list-images"
linkTitle: "list-images"
weight: 10
description: >
    list-images command
---
-->

## Command
<!-- BEGIN SECTION "list-images" "Usage" false -->
Usage: kluctl list-images [flags]

Renders the target and outputs all images used via 'images.get_image(...)
The result is a compatible with yaml files expected by --fixed-images-file.

If fixed images ('-f/--fixed-image') are provided, these are also taken into account,
as described in the deploy command.

<!-- END SECTION -->

## Arguments
The following sets of arguments are available:
1. [project arguments](./common-arguments.md#project-arguments)
1. [image arguments](./common-arguments.md#image-arguments)
1. [inclusion/exclusion arguments](./common-arguments.md#inclusionexclusion-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "list-images" "Misc arguments" true -->
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
      --offline-kubernetes                          Run list-images in offline mode, meaning that it will not try
                                                    to connect the target cluster
  -o, --output stringArray                          Specify output target file. Can be specified multiple times
      --render-output-dir string                    Specifies the target directory to render the project into. If
                                                    omitted, a temporary directory is used.
      --simple                                      Output a simplified version of the images list

```
<!-- END SECTION -->
