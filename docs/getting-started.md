# Getting started

This is a short documentation on how to get started before actually deploying your own infrastructure and/or applications.

## Additional tools needed

You need to install a set of command line tools to fully use kluctl. These are:

1. [helm](https://github.com/helm/helm/releases) <br>
   Download the binaries for your system, make them executable and make them globally available
   (modify your PATH or copy it into /usr/local/bin)
   
All of these tools must be in your PATH, so that kluctl can easily invoke them.

## Get a Kubernetes cluster

The first step is of course: You need a kubernetes cluster. It doesn't really matter where this cluster is hosted, if
it's a managed cluster, or a self-hosted cluster, kops or kubespray based, AWS, GCE, Azure, ... and so on. kluctl
is completely independent of how Kubernetes is deployed and where it is hosted.

There is however a minimum Kubernetes version that must be met: 1.20.0. This is due to the heavy use of server-side apply
which was not stable enough in older versions of Kubernetes.

## Prepare your kubeconfig

Your local kubeconfig should be configured to have access to the Kubernetes cluster via a dedicated context. The context
name should match with the name that you want to use for the cluster from now on. Let's assume the name is `test.example.com`,
then you'd have to ensure that the kubeconfig context `test.example.com` is correctly pointing and authorized for this
cluster.

See [Configure Access to Multiple Clusters](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/) for documentation
on how to manage multiple clusters with a single kubeconfig. Depending on the Kubernets provisioning/deployment tooling
you used, you might also be able to directly export the context into your local kubeconfig. For example,
[kops](https://github.com/kubernetes/kops/blob/master/docs/cli/kops_export.md) is able to export and merge the kubeconfig
for a given cluster.

## Example projects

Now you can play around with the projects within the `examples` folder. In order to have fun with a very simple example, just install [kind](https://kind.sigs.k8s.io/), create a [cluster](https://kind.sigs.k8s.io/docs/user/quick-start/#creating-a-cluster) and run the following commands:
```
cd examples/simple
kluctl diff --target dev
kluctl deploy --target dev
```