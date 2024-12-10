To edit the test.yaml file, run:
```sh
export SOPS_AGE_KEY_FILE=$(pwd)/test-key.txt
sops test.yaml
sops test-configmap.yaml
sops helm-values.yaml
```

To edit the test-gpg.yaml file, run:
```sh
gpg --import private.gpg
sops test-gpg.yaml
```
