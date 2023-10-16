<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "OCI Support"
linkTitle: "OCI Support"
weight: 4
description: >
    OCI Support in Kluctl
---
-->

# OCI Support
Kluctl provides OCI support in multiple places. See the following sections for details.

## Helm OCI based registries
Kluctl fully supports [OCI based Helm registries](https://helm.sh/docs/topics/registries/) in
the [Helm integration](./helm.md).

## OCI includes
Kluctl can include sub-deployments from OCI artifacts via [OCI includes](./deployment-yml.md#oci-includes).

These artifacts can be pushed via the [kluctl oci push](../commands/oci-push.md) sub-command.

## Authentication
Private registries are supported as well. To authenticate to these, use one of the following methods.

### Authenticate via `--registry-xxx` arguments
All [commands](../commands/README.md) that interact with OCI registries support the
[registry arguments](../commands/common-arguments.md#registry-arguments) to specify authentication per OCI registry.

### Authenticate via `docker login`
Kluctl tries to use credentials stored in `$HOME/.docker/config.json` as well, so
[`docker login`](https://docs.docker.com/engine/reference/commandline/login/) will also allow Kluctl to authenticate
against OCI registries.

### Use environment variables to specify authentication
You can also use environment variables to specify OCI authentication.

The following environment variables are supported:

1. `KLUCTL_REGISTRY_HOST`: Specifies the registry host name to match before the specified credentials are considered.
2. `KLUCTL_REGISTRY_REPOSITORY`: Specifies the repository name to match before the specified credentials are considered. The repository name can contain the organization name, which default to `library` is omitted. Can contain wildcards.
3. `KLUCTL_REGISTRY_USERNAME`: Specifies the username.
4. `KLUCTL_REGISTRY_PASSWORD`: Specifies the password.
5. `KLUCTL_REGISTRY_IDENTITY_TOKEN`: Specifies the identity token used for authentication.
6. `KLUCTL_REGISTRY_TOKEN`: Specifies the bearer token used for authentication.
7. `KLUCTL_REGISTRY_INSECURE_SKIP_TLS_VERIFY`: If set to `true`, Kluctl will skip TLS verification for matching registries.
8. `KLUCTL_REGISTRY_PLAIN_HTTP`: If set to `true`, forces the use of http (no TLS).
9. `KLUCTL_REGISTRY_CERT_FILE`: Specifies the client certificate to use while connecting to the matching repository.
10. `KLUCTL_REGISTRY_KEY_FILE`: Specifies the client key to use while connecting to the matching repository.
11. `KLUCTL_REGISTRY_CA_FILE`: Specifies CA bundle to use for TLS/https verification.

Multiple credential sets can be specified by including an index in the environment variable names, e.g.
`KLUCTL_REGISTRY_1_HOST=host.org`, `KLUCTL_REGISTRY_1_USERNAME=my-user` and `KLUCTL_REGISTRY_1_PASSWORD=my-password` will apply
the given credential to all registries with the host `host.org`, while `KLUCTL_REGISTRY_2_HOST=other.org`,
`KLUCTL_REGISTRY_2_USERNAME=my-other-user` and `KLUCTL_REGISTRY_2_PASSWORD=my-other-password` will apply the other credentials
to the `other.org` registry.

### Credentials when using the kluctl-controller
In case you want to use the same Kluctl deployment via the [kluctl-controller](../../gitops/README.md), you have to
configure OCI credentials via [`spec.credentials`](../../gitops/spec/v1beta1/kluctldeployment.md#oci-registry-authentication).
