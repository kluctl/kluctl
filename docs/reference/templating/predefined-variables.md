---
title: "Predefined Variables"
linkTitle: "Predefined Variables"
weight: 1
description: >
    Available predefined variables.
---

There are multiple variables available which are pre-defined by kluctl. These are:

### args
This is a dictionary of arguments given via command line. It contains every argument defined in
[deployment args]({{< ref "docs/reference/deployments/deployment-yml#args" >}}).

### target
This is the target definition of the currently processed target. It contains all values found in the 
[target definition]({{< ref "docs/reference/kluctl-project/targets" >}}), for example `target.name`.

### images
This global object provides the dynamic images features described in [images]({{< ref "docs/reference/deployments/images" >}}).

### version
This global object defines latest version filters for `images.get_image(...)`. See [images]({{< ref "docs/reference/deployments/images" >}}) for details.

### secrets
This global object is only available while [sealing]({{< ref "docs/reference/sealed-secrets" >}}) and contains the loaded
secrets defined via the currently sealed target.
