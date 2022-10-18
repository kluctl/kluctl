---
title: "list-images"
linkTitle: "list-images"
weight: 10
description: >
    list-images command
---

## Command
<!-- BEGIN SECTION "list-images" "Usage" false -->
Usage: kluctl list-images [flags]

Renders the target and outputs all images used via 'images.get_image(...)
The result is a compatible with yaml files expected by --fixed-images-file.

If fixed images ('-f/--fixed-image') are provided, these are also taken into account,
as described in for the deploy command.

<!-- END SECTION -->

## Arguments
The following sets of arguments are available:
1. [project arguments]({{< ref "./common-arguments#project-arguments" >}})
1. [image arguments]({{< ref "./common-arguments#image-arguments" >}})
1. [inclusion/exclusion arguments]({{< ref "./common-arguments#inclusionexclusion-arguments" >}})

In addition, the following arguments are available:
<!-- BEGIN SECTION "list-images" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  -o, --output stringArray         Specify output target file. Can be specified multiple times
      --render-output-dir string   Specifies the target directory to render the project into. If omitted, a
                                   temporary directory is used.
      --simple                     Output a simplified version of the images list

```
<!-- END SECTION -->
