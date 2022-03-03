# kluctl

![kluctl](logo/kluctl.png)

kluctl is the missing glue that puts together your (and any third-party) deployments into one large declarative
Kubernetes deployment, while making it fully manageable (deploy, diff, prune, delete, ...) via one unified command
line interface.

Use kluctl to:
* Organize large and complex deployments, consisting of many Helm charts and kustomize deployments
* Do the same for small and simple deployments, as the overhead is small
* Always know what the state of your deployments is by being able to run diffs on the whole deployment
* Always know what you actually changed after performing a deployment
* Keep your clusters clean by issuing regular prune calls
* Deploy the same deployment to multiple environments (dev, test, prod, ...), with flexible differences in configuration
* Manage multiple target clusters (in multiple clouds or bare-metal if you want)
* Manage encrypted secrets for multiple target environments and clusters (based on [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets))
* Integrate it into your CI/CI pipelines and avoid putting too much logic into your shell scripts

kluctl tries to be as flexible as possible, while keeping it as simple as possible. It reuses established
tools (e.g. kustomize and Helm), making it possible to re-use a large set of available third-party deployments.

kluctl works completely local. In its simplest form, there is no need for any operators or other server-side components.
As long as the target cluster kubeconfig is present locally, you are able to execute it from everywhere, including your
CI/CD pipelines or your laptop.

## Motivation/History

kluctl was created after multiple incarnations of complex multi-environment (e.g. dev, test, prod) deployments, including everything
from monitoring, persistency and the actual custom services. The philosophy of these deployments was always
"what belongs together, should be put together", meaning that only as much Git repositories were involved as necessary.

The problems to solve turned out to be always the same:
* Dozens of Helm Charts, kustomize deployments and standalone Kubernetes deployments needed to be orchestrated in a way
that they work together (services need to connect to the correct databases, and so on)
* (Encrypted) Secrets needed to be managed and orchestrated for multiple environments and clusters
* Updates of components was always risky and required keeping track of what actually changed since the last deployment
* Available tools (Helm, Kustomize) were not suitable to solve this on its own in an easy/natural way
* A lot of bash scripting was required to put things together

When this got more and more complex, and the bash scripts started to become a mess (as "simple" Bash scripts always tend to become),
kluctl was started from scratch. It now tries to solve the mentioned problems and provide a useful set of features (commands)
in a sane way.

## Installation

kluctl can currently only be installed this way:
1. Download a standalone binary from the latest release and make it available in your PATH, either by copying it into `/usr/local/bin` or by modifying the PATH variable

Future releases will include packaged releases for homebrew and other established package managers (contributions are welcome).

## Documentation

### Getting started

You can find a basic Getting Started documentation [here](./docs/getting-started.md).

### kluctl project config (.kluctl.yml)

The [.kluctl.yml](./docs/kluctl_project.md) file is the central configuration file that defines your kluctl project.
It declares what (clusters, secrets, deployment projects, ...) is needed for your deployment, where to find it and what
targets are available to invoke commands against.

### Deployment projects

A deployment project is a collection of actual (kustomize) deployments. Documentation about the project structure and
individual features can be found [here](./docs/deployments.md).

### Command line interface

The command line interface is documented [here](./docs/commands.md)

## Concepts and Features

### Declarative

All deployments are defined in a declarative way. No coding is required to implement deployments that support all
kinds of flavors, target environments, configurations, and so on.

The entrypoint for the deployment is always the `deployment.yml` file, which then declares what else needs to be
included and/or deployed.

### Single source of truth

Deployments realized with kluctl are meant to be the single source of truth for everything that belongs to your
applications. This starts with base infrastructure that needs to be deployed inside Kubernetes, for example ingress
controllers, operators, networking, storage, monitoring ... and then finally leads to your applications, including all third-party
applications (e.g. databases) being deploying with the same tooling.

You are of course free to split up your deployments however you want. For example, it might make sense to split
base infrastructure deployments and application deployments, so that the applications itself get decoupled from the
base infrastructure.

### Jinja2 based templating engine

Kubernetes resources and all other involved configuration is based on [Jinja2](https://palletsprojects.com/p/jinja/)
templates. Jinja2 context variables are usually passed though [kluctl targets](./docs/kluctl_project.md#targets)
but can also be overridden via CLI.

Jinja2 macros allow unifying of heavily repeated deployments (e.g. your 100 microservices) in a convenient way.

### Unified CLI (command line interface)

Deploying your application and all of its dependencies is done via a unified command line interface. It's always
the same, no matter how large (single nginx or 100 microservices) or flexible (test env, uat env, prod env, local env...)
your deployment actually is.

### Secure management of (sealed) secrets in Git

Maintaining secrets inside Git is a complex and dangerous task, but at the same time has many advantages when done
properly. Encrypting such secrets is a must, but there are multiple more or less secure ways to do so.

kluctl has builtin support for [sealed-secrets](https://github.com/bitnami-labs/sealed-secrets). This means,
it can plug in sealed secrets into your deployment in a dynamic and configurable way, targeting multiple clusters,
environments, configurations and so on.

sealed-secrets is public-key crypto based, allowing to target individual clusters or namespaces in a secure way,
meaning that only the targeted environments are able to decrypt secrets. It also means that the private-key needed
for decryption never has to be present while deploying.

### Kustomize integration

Individual deployments are handled by [kustomize](https://kustomize.io/). This allows to run patches and strategic
merges against deployments, which is especially useful when third-party deployments are involved which need
customization. In most cases however, the involved `kustomization.yml` only contains a list kubernetes resources
to deploy.

### Helm integration

A kustomize deployment can also be based on a Helm Chart. Helm Charts can be pulled into the deployment project
and then configured via Jinja2 templating.
