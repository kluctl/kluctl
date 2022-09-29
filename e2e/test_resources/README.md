# sealed-secrets.yaml
```bash
helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm template sealed-secrets-controller sealed-secrets/sealed-secrets -n kube-system --include-crds --skip-tests > sealed-secrets.yaml
```

# vault.yaml
```bash
helm repo add hashicorp https://helm.releases.hashicorp.com
helm template vault hashicorp/vault -n vault -f vault-values.yaml --include-crds --skip-tests > vault.yaml
```

# kluctl-crds.yaml
Fetches flux-kluctl-controller crd definitions
```bash
kustomize build https://github.com/kluctl/flux-kluctl-controller/config/crd > kluctl-crds.yaml
```

# kluctl-deployment.yaml
This is a simple KluctlDeployment that is not really valid. It points to a non-existing GitRepository. This object is only used to test `kluctl flux` subcommands (which only sets annotations and updates field 'suspend').

# flux-source-crd.yaml
Fetches flux-source-controller crd definitions
```bash
kustomize build https://github.com/fluxcd/source-controller/config/crd > flux-source-crd.yaml
```