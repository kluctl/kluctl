To edit the test.yaml file, run:
```sh
SOPS_AGE_KEY_FILE=$(pwd)/test-key.txt sops test.yaml
SOPS_AGE_KEY_FILE=$(pwd)/test-key.txt sops test-configmap.yaml
```
