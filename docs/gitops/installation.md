<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Installation"
linkTitle: "Installation"
weight: 10
description: "Installing the Kluctl Controller"
---
-->

# Installation

The controller can be installed via two available options.

## Using the "install" sub-command

The [`kluctl controller install`](../kluctl/commands/controller-install.md) command can be used to install the
controller. It will use an embedded version of the Controller Kluctl deployment project
found [here](https://github.com/kluctl/kluctl/tree/main/install/controller).

## Using a Kluctl deployment

To manage and install the controller via Kluctl, you can use a Git include in your own deployment:

```yaml
deployments:
  - git:
      url: https://github.com/kluctl/kluctl.git
      subDir: install/controller
      ref:
        tag: v2.23.2
```
