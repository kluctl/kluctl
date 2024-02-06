<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Kluctl GitOps"
linkTitle: "Kluctl GitOps"
description: "Kluctl Controller documentation."
weight: 20
---
-->

# Kluctl GitOps

GitOps in Kluctl is implemented through the Kluctl Controller, which must be [installed](./installation.md)
to your target cluster.

The Kluctl Controller is a Kubernetes operator which implements the [`KluctlDeployment`](spec/v1beta1/kluctldeployment.md#kluctldeployment)
custom resource. This resource allows to define a Kluctl deployment that should be constantly reconciled (re-deployed)
whenever the deployment changes.

## Motivation and Philosophy

Kluctl tries its best to implement all its features via [Kluctl projects](../kluctl/kluctl-project/README.md), meaning that
the deployments are, at least theoretically, deployable from the CLI at all times. The Kluctl Controller does not
add functionality on top of that and thus does not couple your deployments to a running controller.

Instead, the `KluctlDeployment` custom resource acts as an interface to the deployment. It tries to offer the same
functionality and options as offered by the CLI, but through a custom resource instead of a CLI invocation.

As an example, arguments passed via `-a arg=value` can be passed to the custom resource via the `spec.args` field.
The same applies to options like `--dry-run`, which equals to `spec.dryRun: true` in the custom resource. Check the
documentation of [`KluctlDeployment`](spec/v1beta1/kluctldeployment.md#spec-fields) for more such options.

## GitOps Commands

Kluctl GitOps deployments can be controlled via the Kluctl CLI interface, e.g. with
`kluctl gitops deploy --namespace my-ns --name my-deployment`, which will trigger a deployment and wait for it to finish.

See [commands](../kluctl/commands/README.md) for more details.

## Kluctl Webui

The same deployments can also be controlled and monitored via the [Kluctl Webui](../webui/README.md).

## Installation

Installation instructions can be found [here](./installation.md)

## Design

The reconciliation process consists of multiple steps which are constantly repeated:

- **clone** the root Kluctl project via Git
- **prepare** the Kluctl deployment by rendering the whole deployment
- **deploy** the specified target via [kluctl deploy](../kluctl/commands/deploy.md) if the rendered resources changed
- **prune** orphaned objects via [kluctl prune](../kluctl/commands/prune.md)
- **validate** the deployment status via [kluctl validate](../kluctl/commands/validate.md)
- **drift-detection** is performed to allow the [Kluctl Webui](../webui/README.md) to show drift.

Reconciliation is performed on a configurable [interval](spec/v1beta1/kluctldeployment.md#interval). A single
reconciliation iteration will first clone and prepare the project. Only when the rendered resources indicate a change
(by using a hash internally), the controller will initiate a deployment. After the deployment, the controller will
also perform pruning (only if [prune: true](spec/v1beta1/kluctldeployment.md#prune) is set).

When the `KluctlDeployment` is removed from the cluster, the controller cal also delete all resources belonging to
that deployment. This will only happen if [delete: true](spec/v1beta1/kluctldeployment.md#delete) is set.

Deletion and pruning is based on the [discriminator](../kluctl/kluctl-project/README.md#discriminator) of the given target.

A `KluctlDeployment` can be [suspended](spec/v1beta1/kluctldeployment.md#suspend). While suspended, the controller
will skip reconciliation, including deletion and pruning.

The API design of the controller can be found at [kluctldeployment.gitops.kluctl.io/v1beta1](spec/v1beta1/README.md).

## Example

After installing the Kluctl Controller, we can create a `KluctlDeployment` that automatically deploys the
[Microservices Demo](https://kluctl.io/docs/tutorials/microservices-demo/3-templating-and-multi-env/).

Create a KluctlDeployment that uses the demo project source to deploy the `test` target to the same cluster that the
controller runs on.

```yaml
apiVersion: gitops.kluctl.io/v1beta1
kind: KluctlDeployment
metadata:
  name: microservices-demo-test
  namespace: kluctl-system
spec:
  interval: 10m
  source:
    git:
      url: https://github.com/kluctl/kluctl-examples.git
      path: "./microservices-demo/3-templating-and-multi-env/"
  timeout: 2m
  target: test
  context: default
  prune: true
```

This example will deploy a fully-fledged microservices application with multiple backend services, frontends and
databases, all via one single `KluctlDeployment`.

To deploy the same Kluctl project to another target (e.g. prod), simply create the following resource.

```yaml
apiVersion: gitops.kluctl.io/v1beta1
kind: KluctlDeployment
metadata:
  name: microservices-demo-prod
  namespace: kluctl-system
spec:
  interval: 10m
  source:
    git:
      url: https://github.com/kluctl/kluctl-examples.git
      path: "./microservices-demo/3-templating-and-multi-env/"
  timeout: 2m
  target: prod
  context: default
  prune: true
```
