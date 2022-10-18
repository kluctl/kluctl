---
title: "Get Started with Kluctl"
linkTitle: "Get Started"
description: "Get Started with Kluctl."
weight: 20
---

This tutorial shows you how to bootstrap Flux to a Kubernetes cluster and deploy a sample application in a GitOps manner.

## Before you begin

A few things must be prepared before you actually begin.

### Get a Kubernetes cluster

The first step is of course: You need a kubernetes cluster. It doesn't really matter where this cluster is hosted, if
it's a local (e.g. [kind](https://kind.sigs.k8s.io/docs/user/quick-start/)) cluster, managed cluster, or a self-hosted
cluster, kops or kubespray based, AWS, GCE, Azure, ... and so on. kluctl
is completely independent of how Kubernetes is deployed and where it is hosted.

There is however a minimum Kubernetes version that must be met: 1.20.0. This is due to the heavy use of server-side apply
which was not stable enough in older versions of Kubernetes.

### Prepare your kubeconfig

Your local kubeconfig should be configured to have access to the target Kubernetes cluster via a dedicated context. The context
name should match with the name that you want to use for the cluster from now on. Let's assume the name is `test.example.com`,
then you'd have to ensure that the kubeconfig context `test.example.com` is correctly pointing and authorized for this
cluster.

See [Configure Access to Multiple Clusters](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) for documentation
on how to manage multiple clusters with a single kubeconfig. Depending on the Kubernets provisioning/deployment tooling
you used, you might also be able to directly export the context into your local kubeconfig. For example,
[kops](https://github.com/kubernetes/kops/blob/master/docs/cli/kops_export.md) is able to export and merge the kubeconfig
for a given cluster.

## Objectives

- Checkout one of the example Kluctl projects
- Deploy to your local cluster
- Change something and re-deploy

## Install Kluctl

The `kluctl` command-line interface (CLI) is required to perform deployments.

To install the CLI with Homebrew run:

```sh
brew install kluctl/tap/kluctl
```

For other installation methods, see the [install documentation]({{< ref "docs/installation" >}}).

## Clone the kluctl examples

Clone the example project found at https://github.com/kluctl/kluctl-examples

```sh
git clone https://github.com/kluctl/kluctl-examples.git
```

## Choose one of the examples

You can choose whatever example you like from the clones repository. We will however continue this guide by referring
to the `simple-helm` example found in that repository. Change the current directory:

```sh
cd kluctl-examples/simple-helm
```

## Create your local cluster

Create a local cluster with [kind](https://kind.sigs.k8s.io):

```sh
kind create cluster
```

This will update your kubeconfig to contain a context with the name `kind-kind`. By default, all examples will use
the currently active context.

## Deploy the example

Now run the following command to deploy the example:

```sh
kluctl deploy -t simple-helm
```

Kluctl will perform a diff first and then ask for your confirmation to deploy it. In this case, you should only see
some objects being newly deployed.

```sh
kubectl -nsimple-helm get pod
```

## Change something and re-deploy

Now change something inside the deployment project. You could for example add `replicaCount: 2` to `deployment/nginx/helm-values.yml`.
After you have saved your changes, run the deploy command again:

```sh
kluctl deploy -t simple-helm
```

This time it should show your modifications in the diff. Confirm that you want to perform the deployment and then verify
it:

```sh
kubectl -nsimple-helm get pod
```

You should need 2 instances of the nginx POD running now.

## Where to continue?

Continue by reading through the [tutorials]({{< ref "docs/guides/tutorials" >}}) and by consulting
the [reference documentation]({{< ref "reference" >}}).
