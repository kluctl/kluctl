---
title: "Philosophy"
linkTitle: "Philosophy"
weight: 30
description: "The philosophy behind kluctl."
---

Kluctl tries to follow a few basic ideas and a philosophy. Project and deployments structure, as well as all commands
are centered on these.

## Be practical
Everything found in kluctl is based on years of experience in daily business, from the perspective of a DevOps Engineer. 
Kluctl prefers practicability when possible, trying to make the daily life of a DevOps Engineer as comfortable as possible.

## Consistent CLI
Commands try to be as consistent as possible, making it easy to remember how they are used. For example, a `diff` is used the same way as a `deploy`. This applies to all sizes and complexities of projects. A simple/single-application deployment is used the same way as a complex one, so that it is easy to switch between projects.

## Mostly declarative
Kluctl tries to be declarative whenever possible, but loosens this in some cases to stay practical.
For example, hooks, barriers and waitReadiness allows you to control order of deployments in a way that a pure declarative approach would not allow.

## Predictable and traceable
Always know what will happen (`diff` or `--dry-run`) and always know what happened (output changes done by a command).
There is nothing worse than not knowing what's going to happen when you deploy the current state to prod. Not knowing what happened is on the same level.

## Live and let live
Kluctl tries to not interfere with any other tools or operators. It achieves this by honoring managed fields in an intelligent way.
Kluctl will never force-apply anything without being told so, it will also always inform you about fields that you lost ownership of.

## CLI/Client first
Kluctl is centered around a unified command line interface and will always prioritize this. This
guarantees that the DevOps Engineer never looses control, even if automation and/or GitOps style operators are being used.

## No scripting
Kluctl tries its best to remove the need for scripts (e.g. Bash) around deployments. It tries to remove the need
for external orchestration of deployment order and/or dependencies.
