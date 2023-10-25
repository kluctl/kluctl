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
<!-- BEGIN SECTION "gitops logs" "GitOps arguments" true -->
```
GitOps arguments:
  Specify gitops flags.

      --context string                   Override the context to use.
  -l, --label-selector string            If specified, KluctlDeployments are searched and filtered by this label
                                         selector.
      --local-source-override-port int   Specifies the local port to which the source-override client should
                                         connect to when running the controller locally.
      --name string                      Specifies the name of the KluctlDeployment.
  -n, --namespace string                 Specifies the namespace of the KluctlDeployment. If omitted, the current
                                         namespace from your kubeconfig is used.

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops logs" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

  -f, --follow                Follow logs after printing old logs.
      --reconcile-id string   If specified, logs are filtered for the given reconcile ID.

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops logs" "Command Results" true -->
```
Command Results:
  Configure how command results are stored.

      --command-result-namespace string   Override the namespace to be used when writing command results. (default
                                          "kluctl-results")

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops logs" "Log arguments" true -->
```
Log arguments:
  Configure logging.

      --log-grouping-time duration   Logs are by default grouped by time passed, meaning that they are printed in
                                     batches to make reading them easier. This argument allows to modify the
                                     grouping time. (default 1s)
      --log-since duration           Show logs since this time. (default 1m0s)
      --log-time                     If enabled, adds timestamps to log lines

```
<!-- END SECTION -->
