<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Container Images"
linkTitle: "Container Images"
weight: 3
description: >
    Dynamic configuration of container images.
---
-->

# Container Images

There are usually 2 different scenarios where Container Images need to be specified:
1. When deploying third party applications like nginx, redis, ... (e.g. via the [Helm integration](./helm.md)). <br>
   * In this case, image versions/tags rarely change, and if they do, this is an explicit change to the deployment. This
     means it's fine to have the image versions/tags directly in the deployment manifests.
2. When deploying your own applications. <br>
   * In this case, image versions/tags might change very rapidly, sometimes multiple times per hour. Having these
     versions/tags directly in the deployment manifests can easily lead to commit spam and hard to manage 
     multi-environment deployments.
     
kluctl offers a better solution for the second case.

## images.get_image()

This is solved via a templating function that is available in all templates/resources. The function is part of the global
`images` object and expects the following arguments:

`images.get_image(image)`

* image
    * The image name/repository. It is looked up the list of fixed images.

The function will lookup the given image in the list of fixed images and return the last match.

Example deployment:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
spec:
  template:
    spec:
      containers:
      - name: c1
        image: "{{ images.get_image('registry.gitlab.com/my-group/my-project') }}"
```

## Fixed images

Fixed images can be configured multiple methods:
1. Command line argument `--fixed-image`
2. Command line argument `--fixed-images-file`
3. Target definition
4. Global 'images' variable

## Command line argument `--fixed-image`

You can pass fixed images configuration via the `--fixed-image` [argument](../commands/common-arguments.md#image-arguments).
Due to [environment variables support](../commands/environment-variables.md) in the CLI, you can also use the
environment variable `KLUCTL_FIXED_IMAGE_XXX` to configure fixed images.

The format of the `--fixed-image` argument is `--fixed-image image<:namespace:deployment:container>=result`. The simplest
example is `--fixed-image registry.gitlab.com/my-group/my-project=registry.gitlab.com/my-group/my-project:1.1.2`.

## Command line argument `--fixed-images-file`

You can also configure fixed images via a yaml file by using `--fixed-images-file /path/to/fixed-images.yaml`.
file:

```yaml
images:
  - image: registry.gitlab.com/my-group/my-project
    resultImage: registry.gitlab.com/my-group/my-project:1.1.2
```

The file must contain a single root list named `images` with each entry having the following form:

```yaml
images:
  - image: <image_name>
    resultImage: <result_image>
    # optional fields
    namespace: <namespace>
    deployment: <kind>/<name>
    container: <name>
```

`image` and `resultImage` are required. All the other fields are optional and allow to specify in detail for which
object the fixed is specified.

## Target definition

The [target](../kluctl-project/targets/README.md#targets) definition can optionally specify an `images` field that can
contain the same fixed images configuration as found in the `--fixed-images-file` file.

## Global 'images' variable

You can also define a global variable named `images` via one of the [variable sources](../templating/variable-sources.md).
This variable must be a list of the same format as the images list in the `--fixed-images-file` file.

This option allows to externalize fixed images configuration, meaning that you can maintain image versions outside
the deployment project, e.g. in another [Git repository](../templating/variable-sources.md#git).
