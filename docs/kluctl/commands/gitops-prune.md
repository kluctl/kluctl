<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "gitops prune"
linkTitle: "gitops prune"
weight: 10
description: >
    webui command
---
-->

## Command
<!-- BEGIN SECTION "gitops prune" "Usage" false -->
Usage: kluctl gitops prune [flags]

Trigger a GitOps prune
This command will trigger an existing KluctlDeployment to perform a reconciliation loop with a forced prune.
It does this by setting the annotation 'kluctl.io/request-prune' to the current time.

You can override many deployment relevant fields, see the list of command flags for details.

<!-- END SECTION -->

## Arguments

The following arguments are available:
<!-- BEGIN SECTION "gitops prune" "GitOps arguments" true -->
```
GitOps arguments:
  Specify gitops flags.

      --context string          Override the context to use.
  -l, --label-selector string   If specified, KluctlDeployments are searched and filtered by this label selector.
      --name string             Specifies the name of the KluctlDeployment.
  -n, --namespace string        Specifies the namespace of the KluctlDeployment. If omitted, the current namespace
                                from your kubeconfig is used.

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops prune" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --abort-on-error              Abort deploying when an error occurs instead of trying the remaining deployments
      --dry-run                     Performs all kubernetes API calls in dry-run mode.
      --force-apply                 Force conflict resolution when applying. See documentation for details
      --force-replace-on-error      Same as --replace-on-error, but also try to delete and re-create objects. See
                                    documentation for more details.
      --no-obfuscate                Disable obfuscation of sensitive/secret data
  -o, --output-format stringArray   Specify output format and target file, in the format 'format=path'. Format can
                                    either be 'text' or 'yaml'. Can be specified multiple times. The actual format
                                    for yaml is currently not documented and subject to change.
      --replace-on-error            When patching an object fails, try to replace it. See documentation for more
                                    details.
      --short-output                When using the 'text' output format (which is the default), only names of
                                    changes objects are shown instead of showing all changes.

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops prune" "Command Results" true -->
```
Command Results:
  Configure how command results are stored.

      --command-result-namespace string   Override the namespace to be used when writing command results. (default
                                          "kluctl-results")

```
<!-- END SECTION -->
<!-- BEGIN SECTION "gitops prune" "Log arguments" true -->
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
<!-- BEGIN SECTION "gitops prune" "GitOps overrides" true -->
```
GitOps overrides:
  Override settings for GitOps deployments.

      --target-context string   Overrides the context name specified in the target. If the selected target does
                                not specify a context or the no-name target is used, --context will override the
                                currently active context.

```
<!-- END SECTION -->