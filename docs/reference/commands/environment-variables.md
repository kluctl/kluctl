<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Environment Variables"
linkTitle: "Environment Variables"
weight: 2
description: >
    Controlling Kluctl via environment variables
---
-->

In addition to arguments, Kluctl can be controlled via a set of environment variables.

## Environment variables as arguments
All options/arguments accepted by kluctl can also be specified via environment variables. The name of the environment
variables always start with `KLUCTL_` and end with the option/argument in uppercase and dashes replaced with
underscores. As an example, `--dry-run` can also be specified with the environment variable
`KLUCTL_DRY_RUN=true`.

## Additional environment variables
A few additional environment variables are supported which do not belong to an option/argument. These are:

1. `KLUCTL_REGISTRY_<idx>_HOST`, `KLUCTL_REGISTRY_<idx>_USERNAME`, and so on. See [registries](../deployments/images.md#supported-image-registries-and-authentication) for details.
2. `KLUCTL_GIT_<idx>_HOST`, `KLUCTL_GIT_<idx>_USERNAME`, and so on.
3. `KLUCTL_SSH_DISABLE_STRICT_HOST_KEY_CHECKING`. Disable ssh host key checking when accessing git repositories.
