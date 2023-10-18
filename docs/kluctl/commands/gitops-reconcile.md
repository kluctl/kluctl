<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "gitops reconcile"
linkTitle: "gitops reconcile"
weight: 10
description: >
    webui command
---
-->

## Command
<!-- BEGIN SECTION "gitops reconcile" "Usage" false -->
Usage: kluctl gitops reconcile [flags]

Trigger a GitOps reconciliation
This command will trigger an existing KluctlDeployment to perform a reconciliation loop. It does this by setting the annotation 'kluctl.io/request-reconcile' to the current time.

<!-- END SECTION -->

## Arguments

The following arguments are available:
<!-- BEGIN SECTION "gitops reconcile" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

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

```
<!-- END SECTION -->
