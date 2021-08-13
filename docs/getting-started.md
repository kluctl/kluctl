# Getting started

This is a short documentation on how to get started before actually deploying your own infrastructure and/or applications.

## Additional tools needed

You need to install a set of command line tools to fully use kluctl. These are:

1. [kustomize](https://kubectl.docs.kubernetes.io/installation/kustomize/) <br>
   It is currently not possible to use the kubectl integration of kustomize. This means, that you must install the
   kustomize binaries. This might change in the future.
1. [kubeseal](https://github.com/bitnami-labs/sealed-secrets/releases) <br>
   Follow the "Client side" instructions for the latest release
1. [helm](https://github.com/helm/helm/releases) <br>
   Download the binaries for your system, make them executable and make them globally available
   (modify your PATH or copy it into /usr/local/bin)
   
All of these tools must be in your PATH, so that kluctl can easily invoke them.

## Get a Kubernetes cluster

The first step is of course: You need a kubernetes cluster. It doesn't really matter where this cluster is hosted, if
it's a managed cluster, or a self-hosted cluster, kops or kubespray based, AWS, GCE, Azure, ... and so on. kluctl
is completely independent of how Kubernetes is deployed and where it is hosted.

There is however a minimum Kubenetes version that must be met: 1.20.0. This is due to the heavy use of server-side apply
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

## Bootstrap deployment

A cluster that is managed by kluctl needs a bootstrap deployment first. This bootstrap deployment installs mandatory
things into the cluster (e.g. the sealed-secrets controller). The bootstrap deployment is located under the kluctl
Git repository in the `bootstrap` directory. You can easily deploy it with this kluctl invocation:

```shell
$ kluctl -C /location/to/clusters -N test.example.com deploy -d /location/to/kluctl/bootstrap 
```

The bootstrap deployment might need to be updated from time to time. Simply re-do the above command whenever a new
kluctl release is available. You can also use all other kluctl commands (diff, purge, delete, ...) with the
bootstrap deployment.
