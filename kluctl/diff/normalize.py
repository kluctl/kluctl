import fnmatch

from kluctl.utils.k8s_object_utils import split_api_version
from kluctl.utils.dict_utils import is_iterable, copy_dict


def nav_dict(d, k):
    dummy = 'dummy-placeholder-for-dot'
    k = k.replace('\\.', dummy)
    k = k.split('.')
    k = [x.replace(dummy, '.') for x in k]

    for i in range(len(k)):
        if d is None:
            return None, k[i], False
        if isinstance(d, dict):
            if k[i] not in d:
                return d, k[i], False
            if i == len(k) - 1:
                return d, k[i], True
            else:
                d = d[k[i]]
        elif is_iterable(d):
            j = int(k[i])
            if j < 0 or j >= len(d):
                return d, j, False
            if i == len(k) - 1:
                return d, j, True
            else:
                d = d[j]
        else:
            return d, None, False


def del_if_exists(d, k):
    d, k, e = nav_dict(d, k)
    if not e:
        return
    del d[k]

def set_if_not_exists(d, k, v):
    d, k, e = nav_dict(d, k)
    if e:
        return
    d[k] = v

def del_if_falsy(d, k):
    d, k, e = nav_dict(d, k)
    if not e:
        return
    if not d[k]:
        del d[k]

def _object_path_iterator(o, path):
    yield path
    if isinstance(o, dict):
        for k, v in o.items():
            for p in _object_path_iterator(v, path + [k]):
                yield p
    elif not isinstance(o, str) and is_iterable(o):
        for i, v in enumerate(o):
            for p in _object_path_iterator(v, path + [str(i)]):
                yield p

def del_matching_path(o, path):
    for p in list(_object_path_iterator(o, [])):
        if fnmatch.fnmatch(".".join(p), path):
            p2 = [x.replace(".", "\\.") for x in p]
            del_if_exists(o, ".".join(p2))

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

