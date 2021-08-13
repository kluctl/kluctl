# Kubernetes Clusters

kluctl is able to perform the same deployment to different clusters, with each cluster optionally having some
individual/specific configuration. For example each cluster might need its own credentials to access AWS Route53
for DNS management. You might also configure very specific things needed by you, for example a dev cluster could
be configured to have only 1 ingress controller replica while the production has 3 replicas.

This cluster configuration is done via a cluster yaml file. The name of the config file must match the desired
cluster name. The cluster name is then used to identify this cluster in all future CLI invocations.

All cluster configurations must be placed into the same directory. kluctl will then look into this directory
and try to find the correct one via the cluster name. The default directory location is `<current-dir>/clusters`
and can be overriden by the `--cluster-dir` argument.

## Minimal cluster config

Let's assume you want to add a cluster with the name "test.example.com". You'd create the file
`<current-dir>/clusters/test.example.com.yml` and fill it with the following minimal configuration:

```yaml
cluster:
  name: test.example.com
  context: test.example.com
```

The `context` refers to the kubeconfig context that is later used to connect to the cluster. This means, that you must
have that same cluster configured in your kubeconfig, referred by the given context name. The name and context do not
have to match, but it is recommended to use the same name.

## Using cluster config in templates

This configuration is later available whenever Jinja2 templates are involved. This means, that you for example can
use `{{ cluster.name }}` to get the cluster name into one of your deployment configurations/resources.

## Custom cluster configuration

The configuration also allows adding as much custom configuration as you need below the `cluster` dictionary.
For example:

```yaml
cluster:
  name: test.example.com
  context: test.example.com
  ingress_config:
    replicas: 1
    external_auth:
      url: https://example.com/auth
```

This is then also available in all Jinja2 templates (e.g. `{{ cluster.ingress_config.replicas }}`). In the above example, it would allow to configure a test cluster
differently from the production cluster when it comes to ingress controller configuration.
