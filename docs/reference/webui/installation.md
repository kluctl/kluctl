<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: Installation
linkTitle: Installation
description: Installing the Kluctl Webui
weight: 20
---
-->

# Installation

The Kluctl Webui can be installed by using a [Git Include](../deployments/deployment-yml.md#git-includes) that refers
to the [webui deployment project](https://github.com/kluctl/kluctl/tree/main/install/webui). Example:

```yaml
deployments:
  - git:
      url: https://github.com/kluctl/kluctl.git
      subDir: install/webui
      ref:
        tag: v2.20.8
```

# Overriding the version

The image version of the Webui can be overriden with the `kluctl_version` arg:

```yaml
deployments:
  - git:
      url: https://github.com/kluctl/kluctl.git
      subDir: install/webui
      ref:
        tag: main
    vars:
      - values:
          args:
            kluctl_version: v2.20.8
```

# Passing arguments

You can pass arbitrary command line arguments to the webui by providing the `webui_args` arg:

```yaml
deployments:
  - git:
      url: https://github.com/kluctl/kluctl.git
      subDir: install/webui
      ref:
        tag: v2.20.8
    vars:
      - values:
          webui_args:
            - --gops-agent
```
