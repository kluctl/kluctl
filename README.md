# kluctl

[![tests](https://github.com/kluctl/kluctl/workflows/tests/badge.svg)](https://github.com/kluctl/kluctl/actions)
[![license](https://img.shields.io/github/license/kluctl/kluctl.svg)](https://github.com/kluctl/kluctl/blob/main/LICENSE)
[![release](https://img.shields.io/github/release/kluctl/kluctl.svg)](https://github.com/kluctl/kluctl/releases)

<img alt="kluctl" src="logo/kluctl.svg" width="200"/>

Kluctl is the missing glue that puts together your (and any third-party) deployments into one large declarative
Kubernetes deployment, while making it fully manageable (deploy, diff, prune, delete, ...) via one unified command
line interface.

Kluctl tries to be as flexible as possible, while remaining as simple as possible. It reuses established
tools (e.g. Kustomize and Helm), making it possible to re-use a large set of available third-party deployments.

Kluctl is centered around "targets", which can be a cluster or a specific environment (e.g. test, dev, prod, ...) on one
or multiple clusters. Targets can be deployed, diffed, pruned, deleted, and so on. The idea is to have the same set of
operations for every target, no matter how simple or complex the deployment and/or target is.

Kluctl does not strictly depend on a controller and allows to use the same deployment wherever you want,
as long as access to the kluctl project and clusters is available. This means, that you can use it from your
local machine, from your CI/CD pipelines or any automation platform/system that allows to call custom tools.

## What can I do with Kluctl?

Kluctl allows you to define a Kluctl project, which in turn defines Kluctl
deployments and sub-deployments. Each Kluctl deployment defines Kustomize deployments.

A Kluctl project also defines targets, which represent your target environments
and/or clusters.

The Kluctl CLI then allows to deploy, diff, prune, delete, ... your deployments.

## GitOps

If you want to follow a pull based [GitOps](https://kluctl.io/docs/gitops/) flow, then you can use the Kluctl
Controller, which then allows you to use `KluctlDeployment` custom resources to define your Kluctl deployments.

Please note: GitOps support was previously implemented via the now deprecated [flux-kluctl-controller](https://github.com/kluctl/flux-kluctl-controller).
Historically, the flux-kluctl-controller depended on the Flux ecosystem (the source-controller to be specific), which
has changed in the meantime, meaning that it runs completely independent and thus is not part of the Flux ecosystem anymore.

## Kluctl Webui

Kluctl also offers a [Webui](https://kluctl.io/docs/webui/) that allows you to visualise and control your Kluctl
deployments. It works for deployments performed by the CLI and for deployments performed via GitOps.

[Here](https://medium.com/kluctl/introducing-the-kluctl-webui-bcd3ea4b264d) is an introduction to the Webui together
with a tutorial.

## Where do I start?

Installation instructions can be found [here](docs/kluctl/installation.md). For a getting started guide, continue
[here](docs/kluctl/get-started.md).

## Community

Check the [community page](https://kluctl.io/community/) for details about the Kluctl community.

In short: We use [Github Issues](https://github.com/kluctl/kluctl/issues) and
[Github Discussions](https://github.com/kluctl/kluctl/discussions) to track and discuss Kluctl related development.
You can also join the #kluctl channel inside the [CNCF Slack](https://slack.cncf.io) to get in contact with other
community members and contributors/developers.

## Documentation

Documentation, news and blog posts can be found on https://kluctl.io.

The underlying documentation is synced from this repo (look into ./docs) to the website whenever something is merged
into main.

## Development and contributions

Please read [DEVELOPMENT](./DEVELOPMENT.md) and [CONTRIBUTIONS](./CONTRIBUTING.md) for details on how the Kluctl project
handles these matters.

## Kluctl in Short

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

## Demo
![](https://kluctl.io/asciinema/kluctl.gif)
