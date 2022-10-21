<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Predefined Variables"
linkTitle: "Predefined Variables"
weight: 1
description: >
    Available predefined variables.
---
-->

# Predefined Variables

There are multiple variables available which are pre-defined by kluctl. These are:

### args
This is a dictionary of arguments given via command line. It contains every argument defined in
[deployment args](../deployments/deployment-yml.md#args).

### target
This is the target definition of the currently processed target. It contains all values found in the 
[target definition](../kluctl-project/targets), for example `target.name`.

### images
This global object provides the dynamic images features described in [images](../deployments/images.md).

### version
This global object defines latest version filters for `images.get_image(...)`. See [images](../deployments/images.md) for details.

### secrets
This global object is only available while [sealing](../sealed-secrets.md) and contains the loaded
secrets defined via the currently sealed target.
