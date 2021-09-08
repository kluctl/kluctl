import fnmatch

from kluctl.utils.dict_nav_utils import del_if_exists, set_if_not_exists, del_if_falsy, del_matching_path
from kluctl.utils.dict_utils import copy_dict
from kluctl.utils.k8s_object_utils import split_api_version


def normalize_env(container):
    env = container.get("env")
    envFrom = container.get("envFrom")
    if env:
        container['env'] = dict((e['name'], e) for e in env)
    if envFrom:
        types = ['configMapRef', 'secretRef']
        m = {}
        for e in container['envFrom']:
            k = None
            for t in types:
                if t in e:
                    k = '%s/%s' % (t, e[t]['name'])
                    break
            if k is None:
                if 'unknown' not in m:
                    m['unknown'] = []
                m['unknown'].append(e)
            else:
                m[k] = e
        container['envFrom'] = m

def normalize_containers(containers):
    for c in containers:
        normalize_env(c)

def normalize_secret_and_configmap(o):
    del_if_falsy(o, 'data')

def normalize_service_account(o):
    new_secrets = []
    for s in o.get("secrets", []):
        if s["name"].startswith("%s-" % o["metadata"]["name"]):
            continue
        new_secrets.append(s)
    o["secrets"] = new_secrets

def normalize_metadata(k8s_cluster, o):
    group, version = split_api_version(o["apiVersion"])
    api_resource = k8s_cluster.get_resource(group, version, o["kind"])

    if 'metadata' not in o:
        return

    m = o['metadata']

    # remove namespace in case the resource is not namespaced
    if 'namespace' in m:
        if api_resource and not api_resource.namespaced:
            del(m['namespace'])

    # We don't care about managedFields when diffing (they just produce noise)
    del_if_exists(m, 'managedFields')
    del_if_exists(m, 'annotations.kubectl\\.kubernetes\\.io/last-applied-configuration')

    # We don't want to see this in diffs
    del_if_exists(m, 'creationTimestamp')
    del_if_exists(m, 'generation')
    del_if_exists(m, 'resourceVersion')
    del_if_exists(m, 'selfLink')
    del_if_exists(m, 'uid')

    # Ensure empty labels/metadata exist
    set_if_not_exists(m, 'labels', {})
    set_if_not_exists(m, 'annotations', {})

def normalize_misc(o):
    # These are random values found in Jobs
    del_if_exists(o, 'spec.template.metadata.labels.controller-uid')
    del_if_exists(o, 'spec.selector.matchLabels.controller-uid')

    del_if_exists(o, 'status')

# Performs some deterministic sorting and other normalizations to avoid ugly diffs due to order changes
def normalize_object(k8s_cluster, o, ignore_for_diffs):
    ns = o['metadata'].get('namespace')
    kind = o['kind']
    name = o['metadata']['name']

    o = copy_dict(o)
    normalize_metadata(k8s_cluster, o)
    normalize_misc(o)
    if o['kind'] in ['Deployment', 'StatefulSet', 'DaemonSet', 'Job']:
        normalize_containers(o['spec']['template']['spec']['containers'])
    elif o['kind'] in ['Secret', 'ConfigMap']:
        normalize_secret_and_configmap(o)
    elif o['kind'] in ['ServiceAccount']:
        normalize_service_account(o)

    def check_match(v, m):
        if v is None:
            return True
        if isinstance(m, list):
            return any(fnmatch.fnmatch(v, x) for x in m)
        return fnmatch.fnmatch(v, m)

    for ifd in ignore_for_diffs:
        ns2 = ifd.get('namespace', '*')
        kind2 = ifd.get('kind', '*')
        name2 = ifd.get('name', '*')
        field_path = ifd.get('fieldPath')
        if not check_match(ns, ns2):
            continue
        if not check_match(kind, kind2):
            continue
        if not check_match(name, name2):
            continue
        if not isinstance(field_path, list):
            field_path = [field_path]
        for p in field_path:
            del_matching_path(o, p)

    return o

