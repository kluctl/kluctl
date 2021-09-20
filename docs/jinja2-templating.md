# Jinja2 Templating

kluctl uses a Jinja2 Templating engine to pre-process/render every involved configuration file and resource before
actually interpreting it. Only files that are explicitly excluded via [templateExcludes](./deployments.md#templateexcludes)
are not rendered via Jinja2.

Generally, everything that is possible with Jinja2 is possible in kluctl configuration/resources. Please
read into the [Jinja2 documentation](https://jinja.palletsprojects.com/en/3.0.x/templates/) to understand what exactly
is possible and how to use it.

## vars from deployment.yml

[vars](./deployments.md#vars) can be used to add more variables to the deployment project. These are then available
for every kustomize deployment of the current deployment project and also inside all included sub-deployments.

These are however not available while the `deployment.yml` is rendered that defines these vars. This file is
rendered and interpreted before the `vars` are processed and added to the Jinja2 context.

However, each file in `vars` can use all variables defined before that specific file is processed. Consider the
following example.

```yaml
vars:
- file: vars1.yml
- file: vars2.yml
```

`vars2.yml` can now use variables that are defined in `vars1.yml`. At all times, variables defined by
parents of the current sub-deployment project can be used in the current vars file.

## Global variables
There are multiple variables available which are pre-defined by kluctl. These are:

### cluster
This is the cluster definition as found in the cluster yaml that belongs to the chosen target cluster. See
[cluster config](./cluster-config.md) for details on what this variable contains.

### args
This is a dictionary of arguments given via command line. It contains every argument defined in
[deployment args](./deployments.md#args).

### images
This global object provides the dynamic images features described in [images](./images.md).

### version
This global object defines latest version filters for `images.get_image(...)`. See [images](./images.md) for details.

### secrets
This global object is only available while [sealing](./sealed-secrets.md#jinja2-templating) and contains the loaded
secrets defined via the current sealing run.

## Global filters
In addition to the [builtin Jinja2 filters](https://jinja.palletsprojects.com/en/2.11.x/templates/#list-of-builtin-filters),
kluctl provides a few additional filters:

### b64encode
Encodes the input value as base64. Example: `{{ "test" | b64encode }}` will result in `dGVzdA==`.

### b64decode
Decodes an input base64 encoded string. Example `{{ my.source.var | b64decode }}`.

### from_yaml
Parses a yaml string and returns an object. Please note that json is valid yaml, meaning that you can also use this
filter to parse json.

### to_yaml
Converts a variable/object into its yaml representation. Please note that in most cases the resulting string will not
be properly indented, which will require you to also use the `indent` filter. Example:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config
data:
  config.yaml: |
    {{ my_config | to_yaml | indent(4) }}
```

### to_json
Same as `to_yaml`, but with json as output. Please note that json is always valid yaml, meaning that you can also use
`to_json` in yaml files. Consider the following example:

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
        image: my-image
        env: {{ my_list_of_env_entries | to_json }}
```

This would render json into a yaml file, which is still a valid yaml file. Compare this to how this would have to be
solved with `to_yaml`:

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
        image: my-image
        env:
          {{ my_list_of_env_entries | to_yaml | indent(10) }}
```

The required indention filter is the part that makes this error-prone and hard to maintain. Consider using `to_json`
whenever you can.

### render
Renders the input string with the current Jinja2 context. Example:
```
{% set a="{{ my_var }}" %}
{{ a | render }}
```

## Global functions
In addition to the provided
[builtin global functions](https://jinja.palletsprojects.com/en/2.11.x/templates/#list-of-global-functions),
kluctl also provides a few global functions:

### load_template(file)
Loads the given file into memory, renders it with the current Jinja2 context and then returns it as a string. Example:
```
{% set a=load_template('file.yml') %}
{{ a }}
```

The filename given to `load_template` is treated as relative to the template that is currently rendered.

### get_var(field_path, default)
Convenience method to navigate through the current context variables via a
[JSON Path](https://goessner.net/articles/JsonPath/). Let's assume you currently have these variables defines 
(e.g. via [vars](./deployments.md#vars)):
```yaml
my:
  deep:
    var: value
```
Then `{{ get_var('my.deep.var', 'my-default') }}` would return `value`.
When any of the elements inside the field path are non-existent, the given default value is returned instead.

The `field_path` parameter can also be a list of pathes, which are then tried one after the another, returning the first
result that gives a value that is not None. For example, `{{ get_var(['non.existing.var', my.deep.var'], 'my-default') }}`
would also return `value`.

### merge_dict(d1, d2)
Clones d1 and then recursively merges d2 into it and returns the result. Values inside d2 will override values in d1.

### update_dict(d1, d2)
Same as `merge_dict`, but merging is performed in-place into d1.

### raise(msg)
Raises a python exception with the given message. This causes the current command to abort.

### debug_print(msg)
Prints a line to stderr.

## Includes and imports
Standard Jinja2 [includes](https://jinja.palletsprojects.com/en/2.11.x/templates/#include) and
[imports](https://jinja.palletsprojects.com/en/2.11.x/templates/#import) can be used in all templates.

The path given to include/import is treated as relative to the template that is currently rendered.

## Macros

[Jinja2 macros](https://jinja.palletsprojects.com/en/2.11.x/templates/#macros) are fully supported. When writing
macros that produce yaml resources, you must use the `---` yaml separator in case you want to produce multiple resources
in one go.
