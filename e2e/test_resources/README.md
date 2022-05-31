# sealed-secrets.yaml

helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm template sealed-secrets-controller sealed-secrets/sealed-secrets -n kube-system --include-crds > sealed-secrets.yaml
