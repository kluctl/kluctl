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

If an argument needs to be specified multiple times through environment variables, indexed can be appended to the
names of the environment variables, e.g. `KLUCTL_ARG_0=name1=value1` and `KLUCTL_ARG_1=name2=value2`.

## Additional environment variables
A few additional environment variables are supported which do not belong to an option/argument. These are:

1. `KLUCTL_SSH_DISABLE_STRICT_HOST_KEY_CHECKING`. Disable ssh host key checking when accessing git repositories.
