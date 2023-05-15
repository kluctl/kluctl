# sealed-secrets.yaml
```bash
helm repo add sealed-secrets https://bitnami-labs.github.io/sealed-secrets
helm template sealed-secrets-controller sealed-secrets/sealed-secrets -n kube-system --include-crds --skip-tests > sealed-secrets.yaml
```
