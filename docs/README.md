---
title: "Kluctl Documentation"
linkTitle: "Docs"
description: "The missing glue to put together large Kubernetes deployments."
taxonomyCloud: []
weight: 20
menu:
  main:
    weight: 20
---

Kluctl is the missing glue that puts together your (and any third-party) deployments into one large declarative
Kubernetes deployment, while making it fully manageable (deploy, diff, prune, delete, ...) via one unified command
line interface.

Kluctl tries to be as flexible as possible, while remaining as simple as possible. It reuses established
tools (e.g. Kustomize and Helm), making it possible to re-use a large set of available third-party deployments.

Kluctl is centered around "targets", which can be a cluster or a specific environment (e.g. test, dev, prod, ...) on one
or multiple clusters. Targets can be deployed, diffed, pruned, deleted, and so on. The idea is to have the same set of
operations for every target, no matter how simple or complex the deployment and/or target is.

Kluctl does not depend on external operators/controllers and allows to use the same deployment wherever you want,
as long as access to the kluctl project and clusters is available. This means, that you can use it from your 
local machine, from your CI/CD pipelines or any automation platform/system that allows to call custom tools.

Flux support is in alpha state and available via the [flux-kluctl-controller](https://github.com/kluctl/flux-kluctl-controller).

## Kluctl in Short

<!-- borrowed from ./content/en/_index.html -->

|     |     |
| --- | --- |
| 💪 Kluctl handles all your deployments | You can manage all your deployments with Kluctl, including infrastructure related and your applications. |
| 🪶 Complex or simple, all the same | You can manage complex and simple deployments with Kluctl. Simple deployments are lightweight while complex deployment are easily manageable. |
| 🤖 Native git support | Kluctl has native Git support integrated, meaning that it can easily deploy remote Kluctl projects or externalize parts (e.g. configuration) of your Kluctl project. |
| 🪐 Multiple environments | Deploy the same deployment to multiple environments (dev, test, prod, ...), with flexible differences in configuration. |
| 🌌 Multiple clusters | Manage multiple target clusters (in multiple clouds or bare-metal if you want). |
| 🔩 Configuration and Templating | Kluctl allows to use templating in nearly all places, making it easy to have dynamic configuration. |
| ⎈ Helm and Kustomize | The Helm and Kustomize integrations allow you to reuse plenty of third-party charts and kustomizations. |
| 🔍 See what's different | Always know what the state of your deployments is by being able to run diffs on the whole deployment. |
| 🔎 See what happened | Always know what you actually changed after performing a deployment. |
| 💥 Know what went wrong | Kluctl will show you what part of your deployment failed and why. |
| 👐 Live and let live | Kluctl tries to not interfere with any other tools or operators. This is possible due to it's use of server-side-apply. |
| 🧹 Keep it clean | Keep your clusters clean by issuing regular prune calls. |
| 🔐 Encrypted Secrets | Manage encrypted secrets for multiple target environments and clusters. |

## What can I do with Kluctl?

Kluctl allows you to define a Kluctl project, which in turn defines Kluctl
deployments and sub-deployments. Each Kluctl deployment defines Kustomize deployments.

A Kluctl project also defines targets, which represent your target environments
and/or clusters.

The Kluctl CLI then allows to deploy, diff, prune, delete, ... your deployments.

## Where do I start?

{{% alert title="Get started with Kluctl!" %}}
Following this [guide]({{< ref "docs/get-started" >}}) will just take a couple of minutes to complete:
After installing `kluctl`, you can either check out the [example]({{< ref "docs/guides/examples" >}}) or [tutorials]({{< ref "docs/guides/tutorials" >}}).
{{% /alert %}}

<!-- TODO
## Community

Need help or want to contribute? Please see the links below. The Kluctl project is always looking for
new contributors and there are a multitude of ways to get involved.

- Getting Started?
    - Look at our [Get Started guide](get-started/) and give us feedback
- Need help?
    - First: Ask questions on our [GH Discussions page](https://github.com/kluctl/kluctl/discussions)
    - Second: Talk to us in the #kluctl channel on [CNCF Slack](https://slack.cncf.io/)
    - Please follow our [Support Guidelines](/support/)
      (in short: be nice, be respectful of volunteers' time, understand that maintainers and
      contributors cannot respond to all DMs, and keep discussions in the public #kluctl channel as much as possible).
- Have feature proposals or want to contribute?
    - Propose features on our [GH Discussions page](https://github.com/kluctl/kluctl/discussions)
    - Join our upcoming dev meetings ([meeting access and agenda](https://docs.google.com/document/d/1l_M0om0qUEN_NNiGgpqJ2tvsF2iioHkaARDeh6b70B0/view))
    - [Join the flux-dev mailing list](https://lists.cncf.io/g/cncf-kluctl-dev).
    - Check out [how to contribute](/contributing) to the project

### Events

Check out our **[events calendar](/#calendar)**,
both with upcoming talks you can attend or past events videos you can watch.

We look forward to seeing you with us!

-->
