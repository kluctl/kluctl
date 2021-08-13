import os
import re

from kluctl.image_registries.generic_registry import GenericRegistry


def init_image_registries():
    ret = []

    default_tlsverify = os.environ.get("KLUCTL_REGISTRY_DEFAULT_TLSVERIFY", "true") in ["True", "true", "yes", "1"]

    generic = GenericRegistry(default_tlsverify)
    ret.append(generic)

    r = re.compile(r"KLUCTL_REGISTRY_(\d+)_.*")

    registry_indexes = set()
    for env_name in os.environ.keys():
        m = r.match(env_name)
        if m:
            registry_indexes.add(m.group(1))

    def add_registry(base_name):
        host = os.environ.get(f"{base_name}_HOST")
        username = os.environ.get(f"{base_name}_USERNAME")
        password = os.environ.get(f"{base_name}_PASSWORD")
        tlsverify = os.environ.get(f"{base_name}_TLSVERIFY")
        if tlsverify is not None:
            tlsverify = tlsverify in ["True", "true", "yes", "1"]
        if username and password:
            generic.add_creds(host, username, password, tlsverify)

    add_registry(f"KLUCTL_REGISTRY")
    for idx in registry_indexes:
        add_registry(f"KLUCTL_REGISTRY_{idx}")

    return ret
