<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "gitops suspend"
linkTitle: "gitops suspend"
weight: 10
description: >
    webui command
---
-->

## Command
<!-- BEGIN SECTION "gitops suspend" "Usage" false -->
Usage: kluctl gitops suspend [flags]

Suspend a GitOps deployment
This command will suspend a GitOps deployment by setting spec.suspend to 'true'.

<!-- END SECTION -->

## Arguments

The following arguments are available:
<!-- BEGIN SECTION "gitops suspend" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --all                          If enabled, suspend all deployments.
      --context string               Override the context to use.
  -l, --label-selector string        If specified, KluctlDeployments are searched and filtered by this label selector.
      --log-grouping-time duration   Logs are by default grouped by time passed, meaning that they are printed in
                                     batches to make reading them easier. This argument allows to modify the
                                     grouping time. (default 1s)
      --log-since duration           Show logs since this time. (default 1m0s)
      --log-time                     If enabled, adds timestamps to log lines
      --name string                  Specifies the name of the KluctlDeployment.
  -n, --namespace string             Specifies the namespace of the KluctlDeployment. If omitted, the current
                                     namespace from your kubeconfig is used.
      --no-obfuscate                 Disable obfuscation of sensitive/secret data
  -o, --output-format stringArray    Specify output format and target file, in the format 'format=path'. Format
                                     can either be 'text' or 'yaml'. Can be specified multiple times. The actual
                                     format for yaml is currently not documented and subject to change.
      --short-output                 When using the 'text' output format (which is the default), only names of
                                     changes objects are shown instead of showing all changes.

```
<!-- END SECTION -->
