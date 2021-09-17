import logging

from kluctl.utils.dict_utils import get_dict_value
from kluctl.utils.k8s_object_utils import get_included_objects, get_label_from_object, get_object_ref, \
    get_long_object_name_from_ref, split_api_version, get_filtered_api_resources, \
    remove_api_version_from_ref, remove_namespace_from_ref_if_needed
from kluctl.utils.utils import MyThreadPoolExecutor, parse_bool

logger = logging.getLogger(__name__)

# either names or apigroups
high_level_resources_for_delete = [
    "monitoring.coreos.com",
    "kafka.strimzi.io",
    "zookeeper.pravega.io",
    "elasticsearch.k8s.elastic.co",
    "cert-manager.io",
    "bitnami.com",
    "acid.zalan.do",
]

def filter_objects_for_delete(k8s_cluster, objects, api_filter, inclusion, excluded_objects):
    filtered_resources = get_filtered_api_resources(k8s_cluster, api_filter)
    filtered_resources = set((x.group, x.kind) for x in filtered_resources)

    # we must ignore api versions when checking excluded objects as k8s might have changed the version after an object
    # was applied
    excluded_objects = [remove_namespace_from_ref_if_needed(k8s_cluster, x) for x in excluded_objects]
    excluded_objects = [remove_api_version_from_ref(x) for x in excluded_objects]

    inclusion_has_tags = inclusion.has_type("tags")

    def exclude(x):
        group, version = split_api_version(x["apiVersion"])
        if (group or "", x["kind"]) not in filtered_resources:
            return True

        # exclude when explicitly requested
        if parse_bool(get_dict_value(x, 'metadata.annotations."kluctl.io/skip-delete"', "false")):
            return True

        # exclude objects which are owned by some other object
        if 'ownerReferences' in x['metadata'] and len(x['metadata']['ownerReferences']) != 0:
            return True

        if len(get_dict_value(x, "metadata.managedFields", [])) == 0:
            # We don't know who manages it...be safe and exclude it
            return True

        # check if kluctl is managing this object
        if not any([mf['manager'] == 'kluctl' for mf in x['metadata']['managedFields']]):
            # This object is not managed by kluctl, so we shouldn't delete it
            return True

        # exclude objects from excluded_objects
        if remove_api_version_from_ref(get_object_ref(x)) in excluded_objects:
            return True

        # exclude resources which have the 'kluctl.io/skip-delete-if-tags' annotation set
        if inclusion_has_tags:
            # TODO remove label based check
            if get_label_from_object(x, 'kluctl.io/skip_delete_if_tags', 'false') == 'true':
                return True
            if parse_bool(get_dict_value(x, 'metadata.annotations."kluctl.io/skip-delete-if-tags"', "false")):
                return True

        return False

    objects = [x for x in objects if not exclude(x)]
    objects = [get_object_ref(x) for x in objects]
    return objects

def find_objects_for_delete(k8s_cluster, labels, inclusion, excluded_objects):
    logger.info("Getting all cluster objects matching deleteByLabels")
    all_cluster_objects = get_included_objects(k8s_cluster, ["delete"], labels, inclusion, True)
    all_cluster_objects = [x for x, warnings in all_cluster_objects]

    ret = []
    ret += filter_objects_for_delete(k8s_cluster, all_cluster_objects, ["Namespace"], inclusion, excluded_objects + ret)
    ret += filter_objects_for_delete(k8s_cluster, all_cluster_objects, high_level_resources_for_delete, inclusion, excluded_objects + ret)
    ret += filter_objects_for_delete(k8s_cluster, all_cluster_objects, ['Deployment', 'StatefulSet', 'DaemonSet', 'Service', 'Ingress'], inclusion, excluded_objects + ret)
    ret += filter_objects_for_delete(k8s_cluster, all_cluster_objects, None, inclusion, excluded_objects + ret)
    return ret

def delete_objects(k8s_cluster, object_refs, do_wait):
    namespaces = set(x for x in object_refs if x.kind == "Namespace")
    namespace_names = set(x.name for x in namespaces)

    def do_delete_object(ref):
        logger.info('Deleting %s' % get_long_object_name_from_ref(ref))
        k8s_cluster.delete_single_object(ref, do_wait)


    with MyThreadPoolExecutor(max_workers=8) as executor:
        futures = []
        for ref in namespaces:
            f = executor.submit(do_delete_object, ref)
            futures.append(f)

        for ref in object_refs:
            if ref in namespaces:
                continue
            if ref.namespace in namespace_names:
                continue

            f = executor.submit(do_delete_object, ref)
            futures.append(f)

        for f in futures:
            try:
                f.result()
            except Exception as e:
                logger.error('Failed to delete %s. Error=%s' % (get_long_object_name_from_ref(ref), k8s_cluster.get_status_message(e)))
