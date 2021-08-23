import dataclasses
import logging
from typing import Optional

from kubernetes.dynamic.exceptions import ResourceNotFoundError

from kluctl.utils.utils import MyThreadPoolExecutor

logger = logging.getLogger(__name__)

@dataclasses.dataclass(frozen=True)
class ObjectRef:
    api_version: Optional[str] = None
    kind: str = None
    name: str = None
    namespace: Optional[str] = None

def split_api_version(api_version):
    if not api_version:
        return None, 'v1'
    a = api_version.split('/')
    if len(a) == 2:
        return a[0], a[1]
    elif len(a) == 1:
        return None, a[0]
    elif len(a) == 0:
        return None, 'v1'
    else:
        raise ValueError('%s is not a valid apiVersion' % api_version)

def _get_long_object_name(api_version, kind, name, namespace):
    n = [namespace, api_version, kind, name]
    n = [x for x in n if x]
    return '/'.join(n)

def get_long_object_name(o, include_api_version=True):
    api_version = o["apiVersion"]
    if not include_api_version:
        api_version = None
    return _get_long_object_name(api_version,
                                 o.get('kind'),
                                 o['metadata']['name'],
                                 o['metadata'].get('namespace'))

def get_long_object_name_from_ref(ref):
    return _get_long_object_name(ref.api_version, ref.kind, ref.name, ref.namespace)

def get_object_ref(o) -> ObjectRef:
    api_version = o['apiVersion']
    kind = o['kind']
    name = o['metadata']['name']
    namespace = o['metadata'].get('namespace')
    ref = ObjectRef(api_version=api_version, kind=kind, name=name, namespace=namespace)
    return ref

def get_object_refs(objects):
    return [get_object_ref(o) for o in objects]

def remove_api_version_from_ref(ref):
    g, v = split_api_version(ref.api_version)
    return ObjectRef(api_version=g, kind=ref.kind, name=ref.name, namespace=ref.namespace)

def remove_namespace_from_ref_if_needed(k8s_cluster, ref):
    g, v = split_api_version(ref.api_version)

    try:
        resource = k8s_cluster.get_preferred_resource(g, ref.kind)
        if not resource.namespaced:
            return ObjectRef(api_version=ref.api_version, kind=ref.kind, name=ref.name)
        return ref
    except ResourceNotFoundError:
        # resource might be unknown by now as we might be in the middle of deploying CRDs
        return ref

def get_filtered_api_resources(k8s_cluster, filter):
    api_resources = []
    for r in k8s_cluster.get_preferred_resources(None, None, None):
        if filter and r.name not in filter and r.group not in filter and r.kind not in filter:
            continue
        api_resources.append(r)
    return api_resources

def get_objects_metadata(k8s_cluster, verbs, labels):
    api_resources = k8s_cluster.get_preferred_resources(None, None, None)

    with MyThreadPoolExecutor(max_workers=8) as executor:
        futures = []
        for x in api_resources:
            if any(v not in x.verbs for v in verbs):
                continue
            f = executor.submit(k8s_cluster.get_objects_metadata, x.group, x.api_version, x.kind, labels=labels)
            futures.append(f)

        ret = []
        for f in futures:
            ret += f.result()

    return ret

def get_included_objects(k8s_cluster, verbs, labels, inclusion, exclude_if_not_included=False):
    resources = get_objects_metadata(k8s_cluster, verbs, labels)

    ret = []
    for r, warnings in resources:
        inclusion_values = get_tags_from_object(r)
        inclusion_values = [("tag", tag) for tag in inclusion_values]

        kustomize_dir = r.get("metadata", {}).get("annotations", {}).get("kluctl.io/kustomize_dir", None)
        if kustomize_dir is not None:
            inclusion_values.append(("kustomize_dir", kustomize_dir))

        if inclusion.check_included(inclusion_values, exclude_if_not_included):
            ret.append((r, warnings))

    return ret

def get_label_from_object(resource, name, default=None):
    if 'labels' not in resource['metadata']:
        return default
    if name not in resource['metadata']['labels']:
        return default
    return resource['metadata']['labels'][name]

def get_tags_from_object(resource):
    if 'labels' not in resource['metadata']:
        return {}

    tags = {}
    for n, v in resource['metadata']['labels'].items():
        if n.startswith('kluctl.io/tag-'):
            tags[v] = True

    return tags