import base64
import hashlib
import logging
import os
import tempfile

from kluctl.utils.kubeseal import kubeseal_fetch_cert, kubeseal_raw
from kluctl.utils.utils import copy_dict, get_tmp_base_dir
from kluctl.utils.yaml_utils import yaml_load_file, yaml_dump, yaml_load

logger = logging.getLogger(__name__)

HASH_ANNOTATION = "kluctl.io/sealedsecret-hashes"

class Sealer:
    def __init__(self, cluster_vars, sealed_secrets_namespace, sealed_secrets_controller_name, force_reseal):
        self.cluster_vars = cluster_vars
        self.force_reseal = force_reseal
        self.cert = kubeseal_fetch_cert(cluster_vars['context'],
                                        controller_ns=sealed_secrets_namespace,
                                        controller_name=sealed_secrets_controller_name)

        # Can't use NamedTemporaryFile here, thanks to windows not allowing to open it in parallel
        self.cert_dir = tempfile.TemporaryDirectory(dir=get_tmp_base_dir())
        self.cert_file = os.path.join(self.cert_dir.name, "cert")
        with open(self.cert_file, "wb") as f:
            f.write(self.cert)

    def do_hash(self, key, secret, secret_name, secret_namespace, scope):
        salt = f"{self.cluster_vars['name']}-{secret_name}-{secret_namespace or '*'}-{key}"
        if scope != "strict":
            salt += f"-{scope}"
        h = hashlib.scrypt(password=secret, salt=salt.encode("utf-8"), n=16384, r=8, p=1)
        return h.hex()

    def encrypt_secret(self, secret, secret_name, secret_namespace, scope):
        return kubeseal_raw(self.cert_file, secret, secret_name, secret_namespace, scope)

    def seal_file(self, path, target_file):
        basename = os.path.basename(target_file)
        os.makedirs(os.path.dirname(target_file), exist_ok=True)

        content = yaml_load_file(path)
        annotations = content.get("metadata", {}).get("annotations", {})
        secret_name = content.get("metadata", {}).get("name")
        secret_namespace = content.get("metadata", {}).get("namespace", "default")
        secret_type = content.get("type", "Opaque")
        scope = "strict"
        if annotations.get("sealedsecrets.bitnami.com/namespace-wide") == "true":
            scope = "namespace-wide"
        elif annotations.get("sealedsecrets.bitnami.com/cluster-wide") == "true":
            scope = "cluster-wide"
        elif "sealedsecrets.bitnami.com/scope" in annotations:
            scope = annotations["sealedsecrets.bitnami.com/scope"]

        existing_content = {}
        existing_hashes = {}
        if os.path.exists(target_file):
            existing_content = yaml_load_file(target_file)
            existing_annotations = existing_content.get("metadata", {}).get("annotations", {})
            if HASH_ANNOTATION in existing_annotations:
                existing_hashes = yaml_load(existing_annotations[HASH_ANNOTATION])

        secrets = {}
        for k, v in content.get("data", {}).items():
            secrets[k] = base64.b64decode(v)
        for k, v in content.get("stringData", {}).items():
            secrets[k] = v.encode("utf-8")

        result_secret_hashes = {}
        result_encrypted_secrets = {}
        result = {
            "apiVersion": "bitnami.com/v1alpha1",
            "kind": "SealedSecret",
            "metadata": {
                "name": secret_name,
                "annotations": {
                    "sealedsecrets.bitnami.com/scope": scope,
                    **({"sealedsecrets.bitnami.com/namespace-wide": "true"} if scope == "namespace-wide" else {}),
                    **({"sealedsecrets.bitnami.com/cluster-wide": "true"} if scope == "cluster-wide" else {}),
                },
            },
            "spec": {
                "template": {
                    "type": secret_type,
                    "metadata": copy_dict(content.get("metadata", {}))
                },
                "encryptedData": result_encrypted_secrets
            }
        }
        if secret_namespace is not None:
            result["metadata"]["namespace"] = secret_namespace
        result["spec"]["template"].pop("data", None)
        result["spec"]["template"].pop("stringData", None)

        changed_keys = []
        for k, v in secrets.items():
            hash = self.do_hash(k, v, secret_name, secret_namespace, scope)
            existing_hash = existing_hashes.get(k, "")
            if hash == existing_hash and not self.force_reseal:
                logger.debug(f"Secret {secret_name} and key {k} is unchanged, skipping encryption")
                result_encrypted_secrets[k] = existing_content.get("spec", {}).get("encryptedData", {}).get(k)
            else:
                logger.debug(f"Secret {secret_name} and key {k} has changed, encrypting it")
                result_encrypted_secrets[k] = self.encrypt_secret(v, secret_name, secret_namespace, scope)
                changed_keys.append(k)
            result_secret_hashes[k] = hash
        for k in existing_hashes.keys():
            if k not in result_encrypted_secrets:
                logger.debug(f"Secret {secret_name} and key {k} has beed deleted")
                changed_keys.append(k)

        result["metadata"]["annotations"][HASH_ANNOTATION] = yaml_dump(result_secret_hashes)

        if result == existing_content:
            logger.info(f"Skipped {basename} as it did not change")
            return

        logger.info(f"Sealed {basename}. New/changed/deleted keys: {', '.join(changed_keys)}")

        with open(target_file, mode='w') as f:
            yaml_dump(result, f)
