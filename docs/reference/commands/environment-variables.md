---
title: "Environment Variables"
linkTitle: "Environment Variables"
weight: 2
description: >
    Controlling Kluctl via environment variables
---

In addition to arguments, Kluctl can be controlled via a set of environment variables.

## Environment variables as arguments
All options/arguments accepted by kluctl can also be specified via environment variables. The name of the environment
variables always start with `KLUCTL_` and end with the option/argument in uppercase and dashes replaced with
underscores. As an example, `--project-url=my-project` can also be specified with the environment variable
`KLUCTL_PROJECT_URL=my-project`.

## Additional environment variables
A few additional environment variables are supported which do not belong to an option/argument. These are:

1. `KLUCTL_REGISTRY_<idx>_HOST`, `KLUCTL_REGISTRY_<idx>_USERNAME`, and so on. See [registries]({{< ref "docs/reference/deployments/images#supported-image-registries-and-authentication" >}}) for details.
2. `KLUCTL_SSH_DISABLE_STRICT_HOST_KEY_CHECKING`. Disable ssh host key checking when accessing git repositories.
3. `KLUCTL_NO_THREADS`. Do not use multithreading while performing work. This is only useful for debugging purposes.
4. `KLUCTL_IGNORE_DEBUGGER`. Pretend that there is no debugger attached when automatically deciding if multi-threading should be enabled or not.
