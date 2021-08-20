import os

from kluctl.image_registries.generic_registry import GenericRegistry
from kluctl.utils.env_config_sets import parse_env_config_sets


def init_image_registries():
    ret = []

    default_tlsverify = os.environ.get("KLUCTL_REGISTRY_DEFAULT_TLSVERIFY", "true") in ["True", "true", "yes", "1"]

    generic = GenericRegistry(default_tlsverify)
    ret.append(generic)

    def add_registry(s):
        host = s.get("HOST")
        username = s.get("USERNAME")
        password = s.get("PASSWORD")
        tlsverify = s.get("TLSVERIFY")
        if tlsverify is not None:
            tlsverify = tlsverify in ["True", "true", "yes", "1"]
        if username and password:
            generic.add_creds(host, username, password, tlsverify)

    for idx, s in parse_env_config_sets("KLUCTL_REGISTRY").items():
        add_registry(s)

    return ret
