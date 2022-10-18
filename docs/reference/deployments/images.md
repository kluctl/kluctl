---
title: "Container Images"
linkTitle: "Container Images"
weight: 3
description: >
    Dynamic configuration of container images.
---

There are usually 2 different scenarios where Container Images need to be specified:
1. When deploying third party applications like nginx, redis, ... (e.g. via the [Helm integration]({{< ref "./helm" >}})). <br>
   * In this case, image versions/tags rarely change, and if they do, this is an explicit change to the deployment.
1. When deploying your own applications. <br>
   * In this case, image versions/tags might change very rapidly, sometimes multiple times per hour. It would be too much
     effort and overhead when this would be managed explicitly via your deployment. Even with Jinja2 templating, this
     would be hard to maintain.
     
kluctl offers a better solution for the second case.

## Dynamic versions/tags

kluctl is able to ask the used container registry for a list of tags/versions available for an image. It then can
sort the list of images via a configurable order and then use the latest image for your deployment.

It however only does this when the involved resource (e.g. a `Deployment` or `StatefulSet`) is not yet deployed. In case
it is already deployed, the already deployed image will be reused to avoid undesired re-deployment/re-starting of
otherwise unchanged resources.

## images.get_image()

This is solved via a templating function that is available in all templates/resources. The function is part of the global
`images` object and expects the following arguments:

`images.get_image(image, latest_version)`

* image
    * The image location, excluding the tag. Please see [supported image registries](#supported-image-registries-and-authentication) to
      understand which registries are supported.`
* latest_version
    * Configures how tags/versions are sorted and thus how the latest image is determined. Can be:
        * `version.semver()` <br>
          Filters and sorts by loose semantic versioning. Versions must start with a number. It allows unlimited
          `.` inside the version. It treats versions with a suffix as less then versions without a suffix
          (e.g. 1.0-rc1 < 1.0). Two versions which only differ by suffix are sorted semantically.
        * `version.prefix(prefix)` <br>
          Only allows tags with the given prefix and then applies the same logic as images.semver() to whatever
          follows right after the prefix. You can override the handling of the right part by providing `suffix=xxx`,
          while `xxx` is another version filter, e.g. `version.prefix("master-", suffix=version.number())
        * `version.number()` <br>
          Only allows plain numbers as version numbers sorts them accordingly.
        * `version.regex(regex)` <br>
          Only allows versions/tags that match the given regex. Sorting is done the same way as in version.semver(),
          except that versions do not necessarily need to start with a number.

The mentioned version filters must be specified as strings. For example,

`images.get_version("my-image", "prefix('master-', suffix=number())")`.

If no version_filter is specified, then it defaults to `"semver()"`.

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

## Always using the latest images
If you want to use the latest image no matter if an older version is already deployed, use the `-u` flag to your 
`deploy`, `diff` or `list-images` commands.

You can restrict updating to individual images by using `-I/--include-tag`. This is useful when using CI/CD for example,
where you often want to perform a deployment that only updates a single application/service from your large deployment.

## Fixed images via CLI
The described `images.get_image` logic however leads to a loosely defined state on your target cluster/environment. This
might be fine in a CI/CD environment, but might be undesired when deploying to production. In that case, it might be
desirable to explicitly define which versions need to be deployed.

To achieve this, you can use the `-F FIXED_IMAGE` [argument]({{< ref "docs/reference/commands/common-arguments#image-arguments" >}}).
`FIXED_IMAGE` must be in the form of `-F image<:namespace:deployment:container>=result`. For example, to pin the image
`registry.gitlab.com/my-group/my-project` to the tag `1.1.2` you'd have to specify
`-F registry.gitlab.com/my-group/my-project=registry.gitlab.com/my-group/my-project:1.1.2`.

## Fixed images via a yaml file

As an alternative to specifying each fixed image via CLI (`--fixed-images-file=<file>`), you can also specify a single
yaml file via CLI which then contains a list of entries that define image/deployment -> imageResult mappings.

An example fixed-images files looks like this:

```yaml
images:
  - image: registry.gitlab.com/my-group/my-project
    resultImage: registry.gitlab.com/my-group/my-project:1.1.0
  - image: registry.gitlab.com/my-group/my-project2
    resultImage: registry.gitlab.com/my-group/my-project2:2.0.0
  - deployment: StatefulSet/my-sts
    resultImage: registry.gitlab.com/my-group/my-project3:1.0.0
```

You can also take an existing deployment and export the already deployed image versions into a fixed-images file by
using the `list-images` command. It will produce a compatible fixed-images file based on the calls to
`images.get_image` if a deployment would be performed with the given arguments. The result of that call is quite
expressive, as it contains all the information gathered while images were collected. Use `--simple` to only return
a list with image -> resultImage mappings.

## Supported image registries and authentication
All [v2 API](https://docs.docker.com/registry/spec/api/) based image registries are supported, including the Docker Hub,
Gitlab, and many more. Private registries will need credentials to be setup correctly. This can be done by locally
logging in via `docker login <registry>` or by specifying the following environment variables:

Simply set the following environment variables to pass credentials to you private repository:
1. KLUCTL_REGISTRY_HOST=registry.example.com
2. KLUCTL_REGISTRY_USERNAME=username
3. KLUCTL_REGISTRY_PASSWORD=password

You can also pass credentials for more registries by adding an index to the environment variables,
e.g. "KLUCTL_REGISTRY_1_HOST=registry.gitlab.com"

In case your registry uses self-signed TLS certificates, it is currently required to disable TLS verification for these.
You can do this via `KLUCTL_REGISTRY_TLSVERIFY=1`/`KLUCTL_REGISTRY_<idx>_TLSVERIFY=1` for the corresponding
`KLUCTL_REGISTRY_HOST`/`KLUCTL_REGISTRY_<idx>_HOST` or by globally disabling it via `KLUCTL_REGISTRY_DEFAULT_TLSVERIFY=1`.
