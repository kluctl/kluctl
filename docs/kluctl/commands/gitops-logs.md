<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "gitops logs"
linkTitle: "gitops logs"
weight: 10
description: >
    webui command
---
-->

## Command
<!-- BEGIN SECTION "gitops logs" "Usage" false -->
Usage: kluctl gitops logs [flags]

Show logs from controller
Print and watch logs of specified KluctlDeployments from the kluctl-controller.

<!-- END SECTION -->

## Arguments

The following arguments are available:
<!-- BEGIN SECTION "gitops logs" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --context string               Override the context to use.
  -f, --follow                       Follow logs after printing old logs.
  -l, --label-selector string        If specified, KluctlDeployments are searched and filtered by this label selector.
      --log-grouping-time duration   Logs are by default grouped by time passed, meaning that they are printed in
                                     batches to make reading them easier. This argument allows to modify the
                                     grouping time. (default 1s)
      --log-since duration           Show logs since this time. (default 1m0s)
      --log-time                     If enabled, adds timestamps to log lines
      --name string                  Specifies the name of the KluctlDeployment.
  -n, --namespace string             Specifies the namespace of the KluctlDeployment. If omitted, the current
                                     namespace from your kubeconfig is used.
      --reconcile-id string          If specified, logs are filtered for the given reconcile ID.

```
<!-- END SECTION -->
