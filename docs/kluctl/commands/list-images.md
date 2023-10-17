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
1. [helm arguments](./common-arguments.md#helm-arguments)
1. [registry arguments](./common-arguments.md#registry-arguments)

In addition, the following arguments are available:
<!-- BEGIN SECTION "list-images" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --kubernetes-version string   Specify the Kubernetes version that will be assumed. This will also override
                                    the kubeVersion used when rendering Helm Charts.
      --offline-kubernetes          Run command in offline mode, meaning that it will not try to connect the
                                    target cluster
  -o, --output stringArray          Specify output target file. Can be specified multiple times
      --render-output-dir string    Specifies the target directory to render the project into. If omitted, a
                                    temporary directory is used.
      --simple                      Output a simplified version of the images list

```
<!-- END SECTION -->
