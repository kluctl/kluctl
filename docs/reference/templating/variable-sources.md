---
title: "Variable Sources"
linkTitle: "Variable Sources"
weight: 2
description: >
  Available variable sources.
---

There are multiple places in deployment projects (deployment.yaml) where additional variables can be loaded into
future Jinja2 contexts.

The first place where vars can be specified is the deployment root, as documented [here]({{< ref "docs/reference/deployments/deployment-yml#vars-deployment-project" >}}).
These vars are visible for all deployments inside the deployment project, including sub-deployments from includes.

The second place to specify variables is in the deployment items, as documented [here]({{< ref "docs/reference/deployments/deployment-yml#vars-deployment-item" >}}).

The variables loaded for each entry in `vars` are not available inside the `deployment.yaml` file itself.
However, each entry in `vars` can use all variables defined before that specific entry is processed. Consider the
following example.

```yaml
vars:
- file: vars1.yaml
- file: vars2.yaml
```

`vars2.yaml` can now use variables that are defined in `vars1.yaml`. At all times, variables defined by
parents of the current sub-deployment project can be used in the current vars file.

Different types of vars entries are possible:

### file
This loads variables from a yaml file. Assume the following yaml file with the name `vars1.yaml`:
```yaml
my_vars:
  a: 1
  b: "b"
  c:
    - l1
    - l2
```

This file can be loaded via:

```yaml
vars:
  - file: vars1.yaml
```

After which all included deployments and sub-deployments can use the jinja2 variables from `vars1.yaml`.

### values
An inline definition of variables. Example:

```yaml
vars:
  - values:
      a: 1
      b: c
```

These variables can then be used in all deployments and sub-deployments.

### git
This loads variables from a git repository. Example:

```yaml
vars:
  - git:
      url: ssh://git@github.com/example/repo.git
      ref: my-branch
      path: path/to/vars.yaml
```

### clusterConfigMap
Loads a configmap from the target's cluster and loads the specified key's value as a yaml file into the jinja2 variables
context.

Assume the following configmap to be deployed to the target cluster:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-vars
  namespace: my-namespace
data:
  vars: |
    a: 1
    b: "b"
    c:
      - l1
      - l2
```

This configmap can be loaded via:

```yaml
vars:
  - clusterConfigMap:
      name: my-vars
      namespace: my-namespace
      key: vars
```

It assumes that the configmap is already deployed before the kluctl deployment happens. This might for example be
useful to store meta information about the cluster itself and then make it available to kluctl deployments.

### clusterSecret
Same as clusterConfigMap, but for secrets.

### http
The http variables source allows to load variables from an arbitrary HTTP resource by performing a GET (or any other
configured HTTP method) on the URL. Example:

```yaml
vars:
  - http:
      url: https://example.com/path/to/my/vars
```

The above source will load a variables file from the given URL. The file is expected to be in yaml or json format.

The following additional properties are supported for http sources:

##### method
Specifies the HTTP method to be used when requesting the given resource. Defaults to `GET`.

##### body
The body to send along with the request. If not specified, nothing is sent.

#### headers
A map of key/values pairs representing the header entries to be added to the request. If not specified, nothing is added.

##### jsonPath
Can be used to select a nested element from the yaml/json document returned by the HTTP request. This is useful in case
some REST api is used which does not directly return the variables file. Example:

```yaml
vars:
  - http:
      url: https://example.com/path/to/my/vars
      jsonPath: $[0].data
```

The above example would successfully use the following json document as variables source:

```json
[{"data": {"vars": {"var1": "value1"}}}]
```

#### Authentication

Kluctl currently supports BASIC and NTLM authentication. It will prompt for credentials when needed.

### awsSecretsManager
[AWS Secrets Manager](https://aws.amazon.com/secrets-manager/) integration. Loads a variables YAML from an AWS Secrets
Manager secret. The secret can either be specified via an ARN or via a secretName and region combination. An AWS
config profile can also be specified (which must exist while sealing).

The secrets stored in AWS Secrets manager must contain a valid yaml or json file.

Example using an ARN:
```yaml
vars:
  - awsSecretsManager:
      secretName: arn:aws:secretsmanager:eu-central-1:12345678:secret:secret-name-XYZ
      profile: my-prod-profile
```

Example using a secret name and region:
```yaml
vars:
  - awsSecretsManager:
      secretName: secret-name
      region: eu-central-1
      profile: my-prod-profile
```

The advantage of the latter is that the auto-generated suffix in the ARN (which might not be known at the time of
writing the configuration) doesn't have to be specified.

### vault

[Vault by HashiCorp](https://www.vaultproject.io/) with [Tokens](https://www.vaultproject.io/docs/concepts/tokens) 
authentication integration. The address and the path to the secret can be configured. 
The implementation was tested with KV Secrets Engine.

Example using vault:
```yaml
vars:
  - vault:
      address: http://localhost:8200
      path: secret/data/simple
```

Before deploying or sealing please make sure that you have access to vault. You can do this for example by setting 
the environment variable `VAULT_TOKEN`.

### systemEnvVars
Load variables from environment variables. Children of `systemEnvVars` can be arbitrary yaml, e.g. dictionaries or lists.
The leaf values are used to get a value from the system environment.

Example:
```yaml
vars:
- systemEnvVars:
    var1: ENV_VAR_NAME1
    someDict:
      var2: ENV_VAR_NAME2
    someList:
      - var3: ENV_VAR_NAME3
```

The above example will make 3 variables available: `var1`, `someDict.var2` and
`someList[0].var3`, each having the values of the environment variables specified by the leaf values.
