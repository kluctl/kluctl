---
title: "poke-images"
linkTitle: "poke-images"
weight: 10
description: >
    poke-images command
---

## Command
<!-- BEGIN SECTION "poke-images" "Usage" false -->
Usage: kluctl poke-images [flags]

Replace all images in target
This command will fully render the target and then only replace images instead of fully
deploying the target. Only images used in combination with 'images.get_image(...)' are
replaced

<!-- END SECTION -->

## Arguments
The following sets of arguments are available:
1. [project arguments]({{< ref "./common-arguments#project-arguments" >}})
1. [image arguments]({{< ref "./common-arguments#image-arguments" >}})
1. [inclusion/exclusion arguments]({{< ref "./common-arguments#inclusionexclusion-arguments" >}})

In addition, the following arguments are available:
<!-- BEGIN SECTION "poke-images" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --dry-run                     Performs all kubernetes API calls in dry-run mode.
  -o, --output-format stringArray   Specify output format and target file, in the format 'format=path'. Format can
                                    either be 'text' or 'yaml'. Can be specified multiple times. The actual format
                                    for yaml is currently not documented and subject to change.
      --render-output-dir string    Specifies the target directory to render the project into. If omitted, a
                                    temporary directory is used.
  -y, --yes                         Suppresses 'Are you sure?' questions and proceeds as if you would answer 'yes'.

```
<!-- END SECTION -->