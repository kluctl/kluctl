import re

import jsonpatch

from kluctl.utils.dict_utils import set_dict_value, get_dict_value, copy_dict
from kluctl.utils.k8s_object_utils import get_object_ref, remove_api_version_from_ref
from kluctl.utils.utils import parse_bool
from kluctl.utils.yaml_utils import yaml_load

DOWNSCALE_ANNOTATION_PATCH_REGEX = re.compile(r"^kluctl.io/downscale-patch(-\d*)?$")
DOWNSCALE_ANNOTATION_IGNORE = "kluctl.io/downscale-ignore"

def downscale_object(remote_object, local_object):
    if parse_bool(get_dict_value(local_object, "metadata.annotations[\"%s\"]" % DOWNSCALE_ANNOTATION_IGNORE, "false")):
        return remote_object
    for k, v in get_dict_value(local_object, "metadata.annotations", {}).items():
        if DOWNSCALE_ANNOTATION_PATCH_REGEX.fullmatch(k):
            patch = yaml_load(v)
            patch = jsonpatch.JsonPatch(patch)
            remote_object = patch.apply(remote_object)

    ref = get_object_ref(remote_object)
    ref = remove_api_version_from_ref(ref)
    if ref.api_version == "apps" and ref.kind in ["Deployment", "StatefulSet"]:
        return set_dict_value(remote_object, "spec.replicas", 0, True)
    elif ref.api_version == "batch" and ref.kind == "CronJob":
        return set_dict_value(remote_object, "spec.suspend", True)
    #elif ref.api_version == "autoscaling" and ref.kind == "HorizontalPodAutoscaler":
    #    return set_dict_value(o, "spec.minReplicas", 0, True)
    return remote_object
