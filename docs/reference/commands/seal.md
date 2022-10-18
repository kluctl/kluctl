---
title: "seal"
linkTitle: "seal"
weight: 10
description: >
    seal command
---

## Command
<!-- BEGIN SECTION "seal" "Usage" false -->
Usage: kluctl seal [flags]

Seal secrets based on target's sealingConfig
Loads all secrets from the specified secrets sets from the target's sealingConfig and
then renders the target, including all files with the '.sealme' extension. Then runs
kubeseal on each '.sealme' file and stores secrets in the directory specified by
'--local-sealed-secrets', using the outputPattern from your deployment project.

If no '--target' is specified, sealing is performed for all targets.

<!-- END SECTION -->

See [sealed-secrets]({{< ref "docs/reference/sealed-secrets">}}) for more details.

## Arguments
The following sets of arguments are available:
1. [project arguments]({{< ref "./common-arguments#project-arguments" >}}) (except `-a`)

In addition, the following arguments are available:
<!-- BEGIN SECTION "seal" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --cert-file string     Use the given certificate for sealing instead of requesting it from the
                             sealed-secrets controller
      --force-reseal         Lets kluctl ignore secret hashes found in already sealed secrets and thus forces
                             resealing of those.
      --offline-kubernetes   Run seal in offline mode, meaning that it will not try to connect the target cluster

```
<!-- END SECTION -->
