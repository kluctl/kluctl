<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Kluctl"
linkTitle: "Kluctl"
description: >
  Kluctl Documentation
weight: 10
---
-->

# Kluctl

## What is Kluctl?

Kluctl is the missing glue that puts together your (and any third-party) deployments into one large declarative
Kubernetes deployment, while making it fully manageable (deploy, diff, prune, delete, ...) via one unified command
line interface.

## Core Concepts

These are some core concepts in Kluctl.

### Kluctl project
The kluctl project defines targets.
It is defined via the [.kluctl.yaml](./reference/kluctl-project) configuration file.

### Targets
A target defines a target cluster and a set of deployment arguments. Multiple targets can use the same cluster. Targets
allow implementing multi-cluster, multi-environment, multi-customer, ... deployments.

### Deployments
A [deployment](./reference/deployments) defines which Kustomize deployments and which sub-deployments
to deploy. It also controls the order of deployments.

Deployments may be configured through deployment arguments, which are typically provided via the targets but might also
be provided through the CLI.

### Variables
[Variables](./reference/templating) are the main source of configuration. They are either loaded yaml
files or directly defined inside deployments. Each variables file that is loaded has access to all the variables which
were defined before, allowing complex composition of configuration.

After being loaded, variables are usable through the templating engine at all nearly all places.

### Templating
All configuration files (including .kluctl.yaml and deployment.yaml) and all Kubernetes manifests involved are processed
through a templating engine.
The [templating engine](./reference/templating) allows simple variable substitution and also complex
control structures (if/else, for loops, ...).

### Unified CLI
The CLI of kluctl is designed to be unified/consistent as much as possible. Most commands are centered around targets
and thus require you to specify the target name (via `-t <target>`). If you remember how one command works, it's easy
to figure out how the others work. Output from all targets based commands is also unified, allowing you to easily see
what will and what did happen.

## History

Kluctl was created after multiple incarnations of complex multi-environment (e.g. dev, test, prod) deployments, including everything
from monitoring, persistency and the actual custom services. The philosophy of these deployments was always
"what belongs together, should be put together", meaning that only as much Git repositories were involved as necessary.

The problems to solve turned out to be always the same:
* Dozens of Helm Charts, kustomize deployments and standalone Kubernetes manifests needed to be orchestrated in a way
  that they work together (services need to connect to the correct databases, and so on)
* (Encrypted) Secrets needed to be managed and orchestrated for multiple environments and clusters
* Updates of components was always risky and required keeping track of what actually changed since the last deployment
* Available tools (Helm, Kustomize) were not suitable to solve this on its own in an easy/natural way
* A lot of bash scripting was required to put things together

When this got more and more complex, and the bash scripts started to become a mess (as "simple" Bash scripts always tend to become),
kluctl was started from scratch. It now tries to solve the mentioned problems and provide a useful set of features (commands)
in a sane and unified way.

The first versions of kluctl were written in Python, hence the use of Jinja2 templating in kluctl. With version 2.0.0,
kluctl was rewritten in Go.

## Table of Contents

1. [Get Started](./get-started.md)
2. [Installation](./installation.md)
3. [Kluctl Commands](./commands)
4. [.kluctl.yaml](./kluctl-project)
5. [Deployments](./deployments)
