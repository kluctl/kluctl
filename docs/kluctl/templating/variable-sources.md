<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Variable Sources"
linkTitle: "Variable Sources"
weight: 2
description: >
  Available variable sources.
---
-->

# Variable Sources

There are multiple places in deployment projects (deployment.yaml) where additional variables can be loaded into
future Jinja2 contexts.

The first place where vars can be specified is the deployment root, as documented [here](../deployments/deployment-yml.md#vars-deployment-project).
These vars are visible for all deployments inside the deployment project, including sub-deployments from includes.

The second place to specify variables is in the deployment items, as documented [here](../deployments/deployment-yml.md#vars-deployment-item).

The variables loaded for each entry in `vars` are not available inside the `deployment.yaml` file itself.
However, each entry in `vars` can use all variables defined before that specific entry is processed. Consider the
following example.

```yaml
vars:
- file: vars1.yaml
- file: vars2.yaml
- file: optional-vars.yaml
  ignoreMissing: true
- file: default-vars.yaml
  noOverride: true
- file: vars3.yaml
  when: some.var == "value"
- file: vars3.yaml
  sensitive: true
```

`vars2.yaml` can now use variables that are defined in `vars1.yaml`. A special case is the use of previously defined
variables inside [values](#values) vars sources. Please see the documentation of [values](#values) for details.

At all times, variables defined by
parents of the current sub-deployment project can be used in the current vars file.

Each variable source can have the optional field `ignoreMissing` set to `true`, causing Kluctl to ignore if the source
can not be found.

When specifying `noOverride: true`, Kluctl will not override variables from the previously loaded variables. This is
useful if you want to load default values for variables.

Variables can also be loaded conditionally by specifying a condition via `when: <condition>`. The condition must be in
the same format as described in [conditional deployment items](../deployments/deployment-yml.md#when)

Specifying `sensitive: true` causes the Webui to redact the underlying variables for non-admin users. This will be set
to `true` by default for all variable sources that usually load sensitive data, including sops encrypted files and
Kubernetes secrets.

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

Kluctl also supports variable files encrypted with [SOPS](https://github.com/mozilla/sops). See the
[sops integration](../deployments/sops.md) integration for more details.

### values
An inline definition of variables. Example:

```yaml
vars:
  - values:
      a: 1
      b: c
```

These variables can then be used in all deployments and sub-deployments.

In case you need to use variables defined in previous vars sources, the `values` var source needs some special handling
in regard to templating. It's important to understand that the deployment project is rendered BEFORE any vars source
processing is performed, which means that it will fail to render when you use previously defined variables in a `values`
vars source. To still use previously defined variables, surround the `values` vars source with `{% raw %}` and `{% endraw %}`.
In addition, the template expressions must be wrapped with `"`, as otherwise the loading of the deployment project
will fail shortly after rendering due to YAML parsing errors.

```yaml
vars:
  - values:
      a: 1
      b: c
{% raw %}
  - values:
      c: "{{ a }}"
{% endraw %}
```

An alternative syntax is to use a template expression that itself outputs a template expression:

```yaml
vars:
  - values:
      a: 1
      b: c
  - values:
      c: {{ '{{ a }}' }}
```

The advantage of the second method is that the type (number) of `a` is preserved, while the first method would convert
it into a string.

### git
This loads variables from a git repository. Example:

```yaml
vars:
  - git:
      url: ssh://git@github.com/example/repo.git
      ref:
        branch: my-branch
      path: path/to/vars.yaml
```

The ref field has the same format at found in [Git includes](../deployments/deployment-yml.md#git-includes)

Kluctl also supports variable files encrypted with [SOPS](https://github.com/mozilla/sops). See the
[sops integration](../deployments/sops.md) integration for more details.

### clusterConfigMap
Loads a configmap from the target's cluster and loads the specified key's value into the templating context. The value
is treated and loaded as YAML and thus can either be a simple value or a complex nested structure. In case of a simple
value (e.g. a number), you must also specify `targetPath`.

The referred ConfigMap must already exist while the Kluctl project is loaded, meaning that it is not possible to use
a ConfigMap that is deployed as part of the Kluctl project itself.

Assume the following ConfigMap to be already deployed to the target cluster:
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

This ConfigMap can be loaded via:

```yaml
vars:
  - clusterConfigMap:
      name: my-vars
      namespace: my-namespace
      key: vars
```

The following example uses a simple value:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-vars
  namespace: my-namespace
data:
  value: 123
```

This ConfigMap can be loaded via:

```yaml
vars:
  - clusterConfigMap:
      name: my-vars
      namespace: my-namespace
      key: value
      targetPath: deep.nested.path
```

### clusterSecret
Same as clusterConfigMap, but for secrets.

### clusterObject
Retrieves an arbitrary Kubernetes object from the target's cluster and loads the specified content under `path` into the
templating context. The content can either be interpreted as is or interpreted and loaded as yaml text. In both cases,
rendering with the current context (without the newly introduced variables) can also be enabled.

`targetPath` must also be specified to configure under which sub-keys the new variables should be loaded.

The referred Kubernetes object must already exist while the Kluctl project is loaded, meaning that it is not possible to use
an object that is deployed as part of the Kluctl project itself. The exception to this is when you use `ignoreMissing: true`
and properly handle the missing case inside your templating (an example can be found further down).

Objects can either be referred to by `name` or by `labels`. In case of `labels`, Kluctl assumes that only a single object
matches. If multiple object are expected to match, `list: true` must also be passed, in which case the result loaded
into `targetPath` will be a list of objects instead of a single object. 

Assume the following object to be already deployed to the target cluster:
```yaml
apiVersion: some.group/v1
kind: SomeObject
metadata:
  name: my-object
  namespace: my-namespace
spec:
  ...
status:
  my-status: all-good
```

This object can be loaded via:

```yaml
vars:
  - clusterObject:
      kind: SomeObject
      name: my-object
      namespace: my-namespace
      path: status
      targetPath: my.custom.object.status
```

The following properties are supported for clusterObject sources:

##### kind (required)
The object kind. Kluctl will try to find the matching Kubernetes resource for this kind, which might either be a native
API resource or a custom resource. If multiple resources match, `apiVersion` must also be specified.

##### apiVersion (optional)
The apiVersion of the object. This field is only required if `kind` is not enough to identify the underlying API resource.

##### namespace (required)
The namespace from which to load the object.

##### name (optional)
The name of the object. If specified, the object with the given name must exist (`ignoreMissing: true` can override this).

Can be omitted when `labels` is specified.

##### labels (optional)
Specifies one or multiple labels to match. If specified, `name` is not allowed.

By default, assumes and requires (unless `ignoreMissing: true` is set) that only one object matches. If multiple objects
are assumed to match, set `list: true` as well, in which case the result will be a list as well.

##### list (optional)
If set to `true`, the result will be a list with one or more elements.

##### path (required)
Specifies a [JSON path](https://goessner.net/articles/JsonPath/) to be used to load a sub-key from the matching object(s).
Use `$` to load the whole object. To load a single field, use something like `status.my.field`. To load a whole
sub-dict/sub-object or sub-list, use something like `status.conditions`.

The specified JSON path is only allowed to result in a single match.

##### render (optional)
If set to `true`, Kluctl will render the resulting object(s) with the current templating context (excluding the newly
loaded variables). Rendering happens on the values of individual fields of the resulting object(s). When `parseYaml: true`
is specified as well, rendering happens before parsing the YAML string.

##### parseYaml (optional)
Instructs Kluctl to treat the value found at `path` as a YAML string. The value must be of type string. Kluctl will parse
the string as YAML and use the resulting YAML value (which can be a simple int/float/bool or a complex list/dict) as the
result and store it in `targetPath`. When `render: true` is specified as well, the YAML string is rendered before parsing
happens.

##### targetPath (required)
Specifies a [JSON path](https://goessner.net/articles/JsonPath/) to be used as the target path in the new templating
context.

Only simple pathes are supported that do not contain wildcards or lists.

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
Manager secret. The secret can either be specified via an ARN or via a secretName and region combination. An existing AWS
config profile can also be specified.

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

### gcpSecretManager
[Google Secret Manager](https://cloud.google.com/secret-manager) integration. Loads a variables YAML from a Google Secrets
Manager secret. The secret name should be specified in `projects/*/secrets/*/versions/*` [format](https://cloud.google.com/secret-manager/docs/reference/rest/v1/projects.secrets.versions/get#path-parameters).

The secrets stored in Google Secrets manager must contain a valid yaml or json file.

Example:
```yaml
vars:
  - gcpSecretManager:
      secretName: "projects/my-project/secrets/secret/versions/latest"
```

It is recommended to use [workload identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) when you are using kluctl controller. You will need to annotate kluctl controller service account with service account name created in your google project:

```
    args:
      controller_service_account_annotations:
        iam.gke.io/gcp-service-account: kluctl-controller@PROJECT-NAME.iam.gserviceaccount.com
```
substitute PROJECT-NAME with your real project name in google. Service account in your google project should have role `roles/secretmanager.secretAccessor` to access secrets.

To run kluctl locally with gcpSecretManager enabled refer to [setting local development environment](https://cloud.google.com/docs/authentication/provide-credentials-adc#local-dev) article.

### azureKeyVault
[Azure Key Vault](https://azure.microsoft.com/en-us/products/key-vault/) integration.
Loads a variables YAML from an Azure Key Vault.

Example
```yaml
vars:
  - azureKeyVault:
      vaultUri: "https://example.vault.azure.net/"
      secretName: kluctl
```

SDK [azure-sdk-for-go](https://github.com/Azure/azure-sdk-for-go) supports `az login`
or Environment Variables
```bash
  export AZURE_CLIENT_ID="__CLIENT_ID__"
  export AZURE_CLIENT_SECRET="__CLIENT_SECRET__"
  export AZURE_TENANT_ID="__TENANT_ID__"
  export AZURE_SUBSCRIPTION_ID="__SUBSCRIPTION_ID__"
```

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

Before deploying please make sure that you have access to vault. You can do this for example by setting 
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

All specified environment variables must be set before calling kluctl unless a default value is set. Default values
can be set by using the `ENV_VAR_NAME:default-value` form.

Example:
```yaml
vars:
- systemEnvVars:
    var1: ENV_VAR_NAME4:defaultValue
```

The above example will set the variable `var1` to `defaultValue` in case ENV_VAR_NAME4 is not set.

All values retrieved from environment variables (or specified as default values) will be treated as YAML, meaning that
integers and booleans will be treated as integers/booleans. If you want to enforce strings, encapsulate the values in
quotes.

Example:
```yaml
vars:
- systemEnvVars:
    var1: ENV_VAR_NAME5:'true'
```

The above example will treat `true` as a string instead of a boolean. When the environment variable is set outside
kluctl, it should also contain the quotes. Please note that your shell might require escaping to properly pass quotes.
