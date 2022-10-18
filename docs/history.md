---
title: "History"
linkTitle: "History"
weight: 40
description: "The history of kluctl."
---

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
