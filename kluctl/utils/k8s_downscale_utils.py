from kluctl.utils.dict_utils import set_dict_value
from kluctl.utils.k8s_object_utils import get_object_ref, remove_api_version_from_ref


def downscale_object(o):
    ref = get_object_ref(o)
    ref = remove_api_version_from_ref(ref)
    if ref.api_version == "apps" and ref.kind in ["Deployment", "StatefulSet"]:
        return set_dict_value(o, "spec.replicas", 0, True)
    elif ref.api_version == "batch" and ref.kind == "CronJob":
        return set_dict_value(o, "spec.suspend", True)
    #elif ref.api_version == "autoscaling" and ref.kind == "HorizontalPodAutoscaler":
    #    return set_dict_value(o, "spec.minReplicas", 0, True)
    return o
