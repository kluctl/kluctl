<!-- This comment is uncommented when auto-synced to www-kluctl.io

---
title: "Functions"
linkTitle: "Functions"
weight: 4
description: >
    Available functions.
---
-->

# Functions

In addition to the provided
[builtin global functions](https://jinja.palletsprojects.com/en/3.1.x/templates/#list-of-global-functions),
kluctl also provides a few global functions:

### load_template(file)
Loads the given file into memory, renders it with the current Jinja2 context and then returns it as a string. Example:
```
{% set a=load_template('file.yaml') %}
{{ a }}
```

`load_template` uses the same path searching rules as described in [includes/imports](../templating#includes-and-imports).

Please note that there is a limitation in this (and other) functions in regard to loop variables. You can currently not
use loop variables directly as they are not accessible inside Jinja2 extensions/filters. There is an open issue in 
that regard [here](https://github.com/pallets/jinja/issues/1478). For a workaround, perform the same as in
[get_var](#getvarfieldpath-default).

### load_sha256(file, digest_len)
Loads the given file into memory, renders it and calculates the sha256 hash of the result.

The filename given to `load_sha256` is treated the same as in `load_template`. Recursive loading/calculating of hashes
is allowed and is solved by replacing `load_sha256` invocations with currently loaded templates with dummy strings.
This also allows to calculate the hash of the currently rendered template, for example:

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config-{{ load_sha256("configmap.yaml") }}
data:
```

`digest_len` is an optional parameter that allows to limit the length of the returned hex digest.

### load_latest_git_sha(file, sha_len)
Loads the first parent git repository of the given file and returns the latest commit sha1 hash as a hex string.

The filename given to `load_latest_git_sha` is treated the same as in `load_template` and `load_sha256`.

```
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-config-{{ load_latest_git_sha("configmap.yaml") }}
data:
```

`sha_len` is an optional parameter that allows to limit the length of the returned hex digest.

### load_base64(file, width)
Loads the given file into memory and returns the base64 representation of the binary data.
The width parameter is optional and causes `load_base64` to wrap the base64 string into a multiline string.

The filename given to `load_base64` is treated the same as in `load_template`.

This function is useful if you need to include binary data in your deployment. For example:

```
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
data:
  binarySecret: "{{ load_base64("secret.bin") }}"
```

To use wrapped base64, use:

```
apiVersion: v1
kind: Secret
metadata:
  name: my-secret
data:
  binarySecret: |
    "{{ load_base64("large-secret.bin") | indent(4) }}"
```

### get_var(field_path, default)
Convenience method to navigate through the current context variables via a
[JSON Path](https://goessner.net/articles/JsonPath/). Let's assume you currently have these variables defined
(e.g. via [vars](../deployments/deployment-yml.md#vars-deployment-project)):
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

Please note that there is a limitation in this (and other) functions in regard to loop variables. You can currently not
use loop variables directly as they are not accessible inside Jinja2 global functions or filters. There is an open issue in
that regard [here](https://github.com/pallets/jinja/issues/1478). For a workaround, assign the loop variable to a local variable:

```
{% set list=[{x: "a"}, {x: "b"}, {x: "c"}] %}
{% for e in list %}
{% set e=e %} <-- this is the workaround
{{ get_var('e.x') }}
{% endfor %}
```

### merge_dict(d1, d2)
Clones d1 and then recursively merges d2 into it and returns the result. Values inside d2 will override values in d1.

### update_dict(d1, d2)
Same as `merge_dict`, but merging is performed in-place into d1.

### raise(msg)
Raises a python exception with the given message. This causes the current command to abort.

### render(template)
Renders the input string with the current Jinja2 context. Example:
```
{% set a="{{ my_var }}" %}
{{ render(a) }}
```

Please note that there is a limitation in this (and other) functions in regard to loop variables. You can currently not
use loop variables directly as they are not accessible inside Jinja2 global functions or filters. There is an open issue in
that regard [here](https://github.com/pallets/jinja/issues/1478). For a workaround, perform the same as in
[get_var](#getvarfieldpath-default).

### debug_print(msg)
Prints a line to stderr.

### time.now()
Returns the current time. The returned object has the following members:

| member                        | description                                                                                                                                                                        |
|-------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| t.as_timezone(tz)             | Converts and returns the time `t` in the given timezone. Example: <br/>`{{ time.now().as_timezone("Europe/Berlin") }}`                                                             |
| t.weekday()                   | Returns the time's weekday. 0 means Monday and 6 means Sunday.                                                                                                                     |
| t.hour()                      | Returns the time's hour from 0-23.                                                                                                                                                 |
| t.minute()                    | Returns the time's minute from 0-59.                                                                                                                                               |
| t.second()                    | Returns the time's second from 0-59.                                                                                                                                               |
| t.nanosecond()                | Returns the time's nanosecond from 0-999999999.                                                                                                                                    |
| t + delta                     | Adds a delta to `t`. Example: `{{ time.now() + time.second * 10 }}`                                                                                                                |
| t - delta                     | Subtracts a delta from `t`. Example: `{{ time.now() - time.second * 10 }}`                                                                                                         |
| t1 < t2<br/>t1 >= t2<br/> ... | Time objects can be compared to other time objects. Example:<br/>`{% if time.now() < time.parse_iso("2022-10-01T10:00") %}...{% endif %}`<br/>All logical operators are supported. |

### time.utcnow()
Returns the current time in UTC.
The object has the same members as described in [time.now()](#timenow).

### time.parse_iso(iso_time_str)
Parse the given string and return a time object. The string must be in ISO time. 
The object has the same members as described in [time.now()](#timenow).

### time.second, time.minute, time.hour
Represents a time delta to be used with `t + delta` and `t - delta`. Example
```
{{ time.now() + time.minute * 10 }}
```
