# kluctl

![kluctl](logo/kluctl.png)

kluctl is the missing glue that puts together your (and any third-party) deployments into one large declarative
Kubernetes deployment, while making it fully manageable (deploy, diff, prune, delete, ...) via one unified command
line interface.

Use kluctl to:
* Organize large and complex deployments, consisting of many Helm charts and kustomize deployments
* Do the same for small and simple deployments, as the overhead is small
* Always know what the state of your deployments is by being able to run diffs on the whole deployment
* Always know what you actually changed after performing a deployment
* Keep your clusters clean by issuing regular prune calls
* Deploy the same deployment to multiple environments (dev, test, prod, ...), with flexible differences in configuration
* Manage multiple target clusters (in multiple clouds or bare-metal if you want)
* Manage encrypted secrets for multiple target environments and clusters (based on [Sealed Secrets](https://github.com/bitnami-labs/sealed-secrets))
* Integrate it into your CI/CD pipelines and avoid putting too much logic into your shell scripts

kluctl tries to be as flexible as possible, while keeping it as simple as possible. It reuses established
tools (e.g. kustomize and Helm), making it possible to re-use a large set of available third-party deployments.

kluctl works completely local. In its simplest form, there is no need for any operators or other server-side components.
As long as the target cluster kubeconfig is present locally, you are able to execute it from everywhere, including your
CI/CD pipelines or your laptop.

![](https://kluctl.io/asciinema/kluctl.gif)

## Documentation

Documentation can be found here: https://kluctl.io
