<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "gitops deploy"
linkTitle: "gitops deploy"
weight: 10
description: >
    webui command
---
-->

## Command
<!-- BEGIN SECTION "gitops deploy" "Usage" false -->
Usage: kluctl gitops deploy [flags]

Trigger a GitOps deployment
This command will trigger an existing KluctlDeployment to perform a reconciliation loop with a forced deployment. It does this by setting the annotation 'kluctl.io/request-deploy' to the current time.

<!-- END SECTION -->

## Arguments

The following arguments are available:
<!-- BEGIN SECTION "gitops deploy" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --context string          Override the context to use.
  -l, --label-selector string   If specified, KluctlDeployments are searched and filtered by this label selector.
      --name string             Specifies the name of the KluctlDeployment.
  -n, --namespace string        Specifies the namespace of the KluctlDeployment. If omitted, the current namespace
                                from your kubeconfig is used.

```
<!-- END SECTION -->
