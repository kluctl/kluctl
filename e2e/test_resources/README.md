# sealed-secrets.yaml

helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm template sealed-secrets-controller sealed-secrets/sealed-secrets -n kube-system --include-crds --skip-tests > sealed-secrets.yaml

# vault.yaml

helm repo add hashicorp https://helm.releases.hashicorp.com
helm template vault hashicorp/vault -n vault -f vault-values.yaml --include-crds --skip-tests > vault.yaml
