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
This command will trigger an existing KluctlDeployment to perform a reconciliation loop.
It does this by setting the annotation 'kluctl.io/request-reconcile' to the current time.

You can override many deployment relevant fields, see the list of command flags for details.

<!-- END SECTION -->

## Arguments

The following arguments are available:
<!-- BEGIN SECTION "gitops reconcile" "GitOps arguments" true -->
```
GitOps arguments:
  Specify gitops flags.

      --context string                   Override the context to use.
      --controller-namespace string      The namespace where the controller runs in. (default "kluctl-system")
  -l, --label-selector string            If specified, KluctlDeployments are searched and filtered by this label
                                         selector.
      --local-source-override-port int   Specifies the local port to which the source-override client should
                                         connect to when running the controller locally.
      --name string                      Specifies the name of the KluctlDeployment.
  -n, --namespace string                 Specifies the namespace of the KluctlDeployment. If omitted, the current
                                         namespace from your kubeconfig is used.

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops reconcile" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --abort-on-error           Abort deploying when an error occurs instead of trying the remaining deployments
      --dry-run                  Performs all kubernetes API calls in dry-run mode.
      --force-apply              Force conflict resolution when applying. See documentation for details
      --force-replace-on-error   Same as --replace-on-error, but also try to delete and re-create objects. See
                                 documentation for more details.
      --replace-on-error         When patching an object fails, try to replace it. See documentation for more details.

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops reconcile" "Command Results" true -->
```
Command Results:
  Configure how command results are stored.

      --command-result-namespace string   Override the namespace to be used when writing command results. (default
                                          "kluctl-results")

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops reconcile" "Log arguments" true -->
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
<!-- BEGIN SECTION "gitops reconcile" "GitOps overrides" true -->
```
GitOps overrides:
  Override settings for GitOps deployments.

      --no-wait                 Don't wait for objects readiness.
      --prune                   Prune orphaned objects directly after deploying. See the help for the 'prune'
                                sub-command for details.
      --target-context string   Overrides the context name specified in the target. If the selected target does
                                not specify a context or the no-name target is used, --context will override the
                                currently active context.

```
<!-- END SECTION -->