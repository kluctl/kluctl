<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: Legacy Controller Migration
linkTitle: Legacy Controller Migration
description: Legacy Controller Migration
weight: 100
---
-->

# Legacy Controller Migration

Older versions of Kluctl (pre v2.20.0) relied on a legacy version of the Kluctl controller, named
[flux-kluctl-controller](https://github.com/kluctl/flux-kluctl-controller). If you upgraded from such an older
version and were already using `KluctlDeployments` from the `flux.kluctl.io` API group, you must migrate these
deployments to the new `gitops.kluctl.io` group.

To do this, follow the following steps:

1. Upgrade the legacy flux-kluctl-controller to at least v0.16.0. This version will introduce a special marker field
into the legacy `KluctlDeployment` status and set it to true. This marker field is used to inform the new Kluctl Controller
that the legacy controller is now aware of the existence of the new controller.
2. If not already done, [install](../../installation.md#installing-the-gitops-controller) the new Kluctl Controller.
3. To be on the safe side, disable [pruning](https://kluctl.io/docs/flux/spec/v1alpha1/kluctldeployment/#prune) and
[deletion](https://kluctl.io/docs/flux/spec/v1alpha1/kluctldeployment/#delete) for all legacy `KluctlDeployment` objects.
Don't forget to deploy/apply these changes before continuing with the next step.
4. Modify your `KluctlDeployment` manifests to use the `gitops.kluctl.io/v1beta1` as `apiVersion`. It's important
to use the same name and namespace as used in the legacy resources. Also read the [breaking changes](#breaking-changes)
section.
5. Deploy/Apply the modified `KluctlDeployment` resources.
6. At this point, the legacy controller will detect that the `KluctlDeployment` exists twice, once for the legacy
API group/version and once for the new group/version. Based on that knowledge, the legacy controller will stop reconciling
the legacy `KluctlDeployment`.
7. At the same time, the new controller will detect that the legacy `KluctlDeployment` has the marker field set, which
means that the legacy controller is known to honor the new controller's existence.
8. This will lead to the new controller taking over and reconciling the new `KluctlDeployment`.
9. If you disabled deletion/pruning in step 3., you should undo this on the new `KluctlDeployments` now.

After these steps, the legacy `KluctlDeployment` resources will be excluded from reconciliation by the legacy controller.
This means, you can safely remove/prune the legacy resources.

# Breaking changes

There exist some breaking changes between the legacy `flux.kluctl.io/v1alpha1` and `gitops.kluctl.io/v1beta1` custom
resources and controllers. These are:

### Only deploy when resources change

The legacy controller did a full deploy on each reconciliation, following the Flux way of reconciliations. This
behaviour was configurable by allowing you to set `spec.deployInterval: never`, which disabled full deployments and
caused the controller to only deploy when the resulting rendered resources actually changed.

The new controller will behave this way by default, unless you explicitly set `spec.deployInterval` to some interval
value.

This means, you will have to introduce `spec.deployInterval` in case you expect the controller to behave as before or
remove `spec.deployInterval: never` if you already used the Kluctl specific behavior.

### renameContexts has been removed

The `spec.renameContexts` field is not available anymore. Use `spec.context` instead.

### status will not contain full result anymore

The legacy controller wrote the full command result (with objects, diffs, ...) into the status field. The new
controller will instead only write a summary of the result.

# Why no fully automated migration?

I have decided against a fully automated migration as the move of the API group causes resources to have a different
identity. This can easily lead to unexpected behaviour and does not play well with GitOps.
