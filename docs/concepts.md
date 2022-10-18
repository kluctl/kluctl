---
title: Core Concepts
description: Core Concepts of Kluctl.
weight: 10
---

These are some core concepts in Kluctl.

## Kluctl project
The kluctl project defines targets, secret sources and external git projects.
It is defined via the [.kluctl.yaml]({{< ref "docs/reference/kluctl-project" >}}) configuration file.

The kluctl project can also optionally define where the deployment project and clusters configs are located (external
git projects).

## Targets
A target defines a target cluster and a set of deployment arguments. Multiple targets can use the same cluster. Targets
allow implementing multi-cluster, multi-environment, multi-customer, ... deployments.

## Deployments
A [deployment]({{< ref "docs/reference/deployments" >}}) defines which Kustomize deployments and which sub-deployments
to deploy. It also controls the order of deployments.

Deployments may be configured through deployment arguments, which are typically provided via the targets but might also
be provided through the CLI.

## Variables
[Variables]({{< ref "docs/reference/templating" >}}) are the main source of configuration. They are either loaded yaml
files or directly defined inside deployments. Each variables file that is loaded has access to all the variables which
were defined before, allowing complex composition of configuration.

After being loaded, variables are usable through the templating engine at all nearly all places.

## Templating
All configuration files (including .kluctl.yaml and deployment.yaml) and all Kubernetes manifests involved are processed
through a templating engine.
The [templating engine]({{< ref "docs/reference/templating" >}}) allows simple variable substitution and also complex
control structures (if/else, for loops, ...).

## Secrets
Secrets are loaded from [external sources]({{< ref "docs/reference/kluctl-project" >}}) and are only available
while [sealing]({{< ref "docs/reference/sealed-secrets" >}}). After the sealing process, only the public-key encrypted
sealed secrets are available.

## Sealed Secrets
[Sealed Secrets]({{< ref "docs/reference/sealed-secrets" >}}) are based on
[Bitnami's sealed-secrets controller](https://github.com/bitnami-labs/sealed-secrets). Kluctl offers integration of
sealed secrets through the `seal` command. Kluctl allows managing multiple sets of sealed secrets for multiple targets.

## Unified CLI
The CLI of kluctl is designed to be unified/consistent as much as possible. Most commands are centered around targets
and thus require you to specify the target name (via `-t <target>`). If you remember how one command works, it's easy
to figure out how the others work. Output from all targets based commands is also unified, allowing you to easily see
what will and what did happen.
