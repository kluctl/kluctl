# .kluctl.yml project

The `.kluctl.yml` is the central configuration and entry point for your deployments. It defines where the actual
[deployment project](./deployments.md) is located, whre the [cluster configuration](./cluster-config.md) is located,
where [sealed secrets](./sealed-secrets.md) and unencrypted secrets are localed and which targets are available to
invoke [commands](./commands.md) on.

## Example

An example .kluctl.yml looks like this:

```yaml
# This is optional. If omitted, the same directory where `.kluctl.yml` is located will be used as root deployment
deployment:
  project:
    url: https://github.com/codablock/kluctl-example

# This is optional. If omitted, `<baseDirOfKluctlYml>/clusters` will be used
clusters:
  project:
    url: https://github.com/codablock/kluctl-example-clusters
    subdir: clusters

# This is optional. If omitted, `<baseDirOfKluctlYml>/.sealed-secrets` will be used
sealedSecrets:
  project:
    url: https://github.com/codablock/kluctl-example
    subdir: .sealed-secrets

targets:
  # test cluster, dev env
  - name: dev
    cluster: test.example.com
    args:
      environment_name: dev
    sealingConfig:
      secretSets:
        - non-prod
  # test cluster, test env
  - name: test
    cluster: test.example.com
    args:
      environment_name: test
    sealingConfig:
      secretSets:
        - non-prod
  # prod cluster, prod env
  - name: prod
    cluster: prod.example.com
    args:
      environment_name: prod
    sealingConfig:
      secretSets:
        - prod

# This is only required if you actually need sealed secrets
secretsConfig:
  secretSets:
    - name: prod
      sources:
        # This file should not be part of version control!
        - path: .secrets-prod.yml
    - name: non-prod
      sources:
        # This file should not be part of version control!
        - path: .secrets-non-prod.yml
```

## Allowed fields

The following fields are allowed in `.kluctl.yml`:

### deployment

Specifies the git project where the [deployment project](./deployments.md) is located. If it is omitted, the base
directory of the `.kluctl.yml` project is used as the deployment project root.

It has the following form:
```yaml
deployment:
  project:
    url: <git-url>
    ref: <tag-or-branch>
    subdir: <subdir>
```

* `url` specifies the git clone url of the project.
* `ref` is optional and specifies which tag/branch to use. If omitted, the repositories default branch is used.
* `subdir` is optional and specifies the subdirectory to use. If omitted, the repository root is used.

### clusters

Specifies the git project where the [cluster configuration](./cluster-config.md) is located. If it is omitted, the
`clusters` subdirectory of the `.kluctl.yml` project is used as the clusters config root.

It has the same form as in [deployment](#deployment), except that it is called `clusters`.

### sealedSecrets

Specifies the git project where the [sealed secrets](./cluster-config.md) are located. If it is omitted, the
`.sealed-secrets` subdirectory of the `.kluctl.yml` project is used as the sealed secrets location.

It has the same form as in [deployment](#deployment), except that it is called `sealedSecrets`.

### targets

Specifies a list of targets for which commands can be invoked. A target puts together environment/target specific
configuration and the target cluster. Multiple targets can exist which target the same cluster but with differing
configuration (via `args`). Target entries also specifies which secrets to use while [sealing](./sealed-secrets.md).

Each value found in the target definition is rendered with a simple Jinja2 context that only contains the target itself
and cluster configuration. The rendering process is retried 10 times until it finally succeeds, allowing you to reference
the target itself in complex ways. This is especially useful when using [dynamic targets](#dynamic-targets).

Target entries have the following form:
```yaml
targets:
...
  - name: <target_name>
    cluster: <cluster_name>
    args:
      arg1: <value1>
      arg2: <value2>
      ...
    sealingConfig:
      secretSets:
        - <name_of_secrets_set>
...
```

* `name` specifies the name of the target. The name must be unique. It is referred in all commands via the 
  [-t](./commands.md#project-arguments) option.
* `cluster` specifies the name of the target cluster. The cluster must exist in the [cluster configuration](./cluster-config.md)
  specified via [clusters](#clusters).
* `args` specifies a map of arguments to be passed to the deployment project when it is rendered. Allowed argument names
  are configured via [deployment args](./deployments.md#args)
* `sealingConfig` configures how sealing is performed when the [seal command] (./commands.md#seal) is invoked for this target.
  It has the following form:
  ```yaml
  sealingConfig:
    dynamicSealing: <true_or_false>
    args:
      arg1: <override_for_arg1>
    secretSets:
      - <name_of_secrets_set>
  ```
  * `dynamicSealing` specifies weather sealing should happen per [dynamic target](#dynamic-targets) or only once. This 
    field is optional and default to `true`.
  * `args` allows to add extra arguments to the target args. These are only used while sealing and may override
    arguments which are already configured for the target.
  * `secretSets` specifies a list of secret set names, which all must exist in the [secretsConfig](#secretsconfig).

### secretsConfig

This configures how secrets are retrieved while sealing. It is basically a list of named secret sets which can be
referenced from targets.

It has the following form:
```yaml
...
secretsConfig:
  secretSets:
    - name: <name>
      sources:
        - ...
...
```

* `name` specifies the name of the secret set. The name can be used in targets to refer to this secret set.
* `sources` specifies a list of secret sources. See below on which sources are supported.

### Supported sources

#### path
A simple local file based source. The path must be relative and multiple places are tried to find the file:
1. Relative to the deployment project root
2. The path provided via [--secrets-dir](./commands.md#seal)

Example:
```yaml
secretsConfig:
  secretSets:
    - name: prod
      sources:
        - path: .secrets-non-prod.yml
```

#### passwordstate
[Passwordstate](https://www.clickstudios.com.au/passwordstate.aspx) integration. It requires API access rights and will
prompt for login information while sealing. The password entry must have a field with the name given in `passwordField`
(defaults to `GenericField1`) which contains the yaml file with the actual secrets.

Example:
```yaml
secretsConfig:
  secretSets:
    - name: prod
      sources:
        - passwordstate:
            host: passwordstate.example.com
            passwordList: /My/Path/To/The/List/name-of-the-list
            passwordTitle: title-of-the-password
            passwordField: field-name # This is optional and defaults to GenericField1
```


## Dynamic Targets

Targets can also be "dynamic", meaning that additional configuration can be sourced from another git repository.
This can be based on a single target repository and branch, or on a target repository and branch/ref pattern, resulting
in multiple dynamic targets being created from one target definition.

Please note that a single entry in `target` might end up with multiple dynamic targets, meaning that the name must be
made unique between these dynamic targets. This can be achieved by using templating in the `name` field. As an example,
`{{ target.targetConfig.ref }}` can be used to set the target name to the branch name of the dynamic target.

Dynamic targets have the following form:
```yaml
targets:
...
  - name: <dynamic_target_name>
    cluster: <cluster_name>
    args: ...
      arg1: <value1>
      arg2: <value2>
      ...
    targetConfig:
      project:
        url: <git-url>
      ref: <ref-name>
      refPattern: <regex-pattern>
      file: <config-file>
    sealingConfig:
      dynamicSealing: <false_or_true>
      secretSets:
        - <name_of_secrets_set>
...
```

* `name` specifies the name of the dynamic target, same as with normal targets, except that you have to ensure that the
  name is unique between all targets. See above for details.
* `targetConfig` specifies where to look for dynamic targets and their addional configuration. It has the following form:
  ```yaml
  ...
    targetConfig:
      project:
        url: <git-url>
      ref: <ref-name>
      refPattern: <regex-pattern>
  ...
  ```
  * `url` specifies the specifies the git clone url of the target configuration project.
  * `ref` specifies the branch or tag to use. If this is specified, using `refPattern` is forbidden. This will result
     in one single dynamic target.
  * `refPattern` specifies a regex pattern to use when looking for candidate branches and tags. If this is specified,
    using `ref` is forbidden. This will result in multiple dynamic targets. Each dynamic target will have `ref` set to
    the actual branch name it belong to. This allows using of `{{ target.targetConfig.ref }}` in all other target fields.
  * `file` specifies the config file name to read externalized target config from.
* `dynamicSealing` in `sealingConfig` allows to disable sealing per dynamic target and instead reverts to sealing only once.
  See [A note on sealing](#a-note-on-sealing) for details.
* all other fields are the same as for normal targets

### Simple dynamic targets

A simplified form of dynamic targets is to store target config inside the same directory/project as the `.kluctl.yml`.
This can be done by omitting `project`, `ref` and `refPattern` from `targetConfig` and only specify `file`.

### A note on sealing

When sealing dynamic targets, it is very likely that it is not known yet which dynamic targets will actually exist in
the future. This requires some special care when sealing secrets for these targets. Sealed secrets are usually namespace
scoped, which might need to be changed to cluster-wide scoping so that the same sealed secret can be deployed into
multiple targets (assuming you deploy to different namespaces for each target). When you do this, watch out to not
compromise security, e.g. by sealing production level secrets with a cluster-wide scope!

It is also very likely required to set `target.sealingConfig.dynamicSealing` to `false`, so that sealing is only performed
once and not for all dynamic targets.

# Separating kluctl projects and deployment projects

As seen in the `.kluctl.yml` documentation, deployment projects can rely in other repositories then the kluctl project.
This is a desired pattern in some circumstances, for example when you want to share a single deployment project with
multiple teams that all manage their own clusters. This way each team can have its own minimalistic kluct project which
points to the deployment project and the teams clusters configuration.

This way secret sources can also differ between teams and sharing can be reduced to a minimum if desired.
