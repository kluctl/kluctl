<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "controller run"
linkTitle: "controller run"
weight: 10
description: >
    controller command
---
-->

## Command
<!-- BEGIN SECTION "controller run" "Usage" false -->
Usage: kluctl controller run [flags]

Run the Kluctl controller
This command will run the Kluctl Controller. This is usually meant to be run inside a cluster and not from your local machine.

<!-- END SECTION -->

## Arguments

The following arguments are available:
<!-- BEGIN SECTION "controller run" "Misc arguments" true -->
```
Misc arguments:
  Command specific arguments.

      --concurrency int                       Configures how many KluctlDeployments can be be reconciled
                                              concurrently. (default 4)
      --context string                        Override the context to use.
      --controller-name string                The controller name used for metrics and logs. (default
                                              "kluctl-controller")
      --controller-namespace string           The namespace where the controller runs in. (default "kluctl-system")
      --default-service-account string        Default service account used for impersonation.
      --dry-run                               Run all deployments in dryRun=true mode.
      --health-probe-bind-address string      The address the probe endpoint binds to. (default ":8081")
      --kubeconfig string                     Override the kubeconfig to use.
      --leader-elect                          Enable leader election for controller manager. Enabling this will
                                              ensure there is only one active controller manager.
      --metrics-bind-address string           The address the metric endpoint binds to. (default ":8080")
      --namespace string                      Specify the namespace to watch. If omitted, all namespaces are watched.
      --source-override-bind-address string   The address the source override manager endpoint binds to. (default
                                              ":8082")

```
<!-- END SECTION -->
