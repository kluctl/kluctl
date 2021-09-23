import json

from kluctl.utils.k8s_object_utils import get_object_ref, get_long_object_name
from kluctl.utils.dict_utils import copy_dict


def _fields_iterator(mf, path):
    yield path
    for k, v in mf.items():
        for p in _fields_iterator(v, path + [k]):
            yield p

def nav_managed_field(o, p):
    def find_array_value(a, k):
        k = json.loads(k)
        for i in range(len(a)):
            o = a[i]
            match = True
            for n, v in k.items():
                if n not in o or o[n] != v:
                    match = False
                    break
            if match:
                return i
        return -1

    o_in = o

    for i in range(len(p)):
        if p[i] == '.':
            return o, None, False

        k = p[i]
        t = k[0]
        k = k[2:]
        if t == 'f':
            if k not in o:
                return o, k, False
            if i == len(p) - 1:
                return o, k, True
            o = o[k]
        elif t == 'k':
            j = find_array_value(o, k)
            if j == -1:
                return o, -1, False
            if i == len(p) - 1:
                return o, j, True
            o = o[j]
        else:
            raise ValueError('Unsupported managed field %s in object %s' % ('.'.join(p), get_long_object_name(o_in)))

ignored_fields = {
    ("f:metadata",)
}

# We automatically force overwrite these fields as we assume these are human-edited
overwrite_allowed_managers = {
    "kluctl",
    "kubectl",
    "kubectl-edit",
    "kubectl-client-side-apply",
    "rancher",
}

def remove_non_managed_fields(o, managed_fields):
    v1_fields = [mf for mf in managed_fields if mf['fieldsType'] == 'FieldsV1']

    kluctl_fields = None
    for mf in v1_fields:
        if mf['manager'] in ['kluctl'] and mf['operation'] == 'Apply':
            kluctl_fields = mf
            break
    if kluctl_fields is None:
        # Can't do anything with this object...let's hope it will not conflict
        return o

    kluctl_fields = set(tuple(p) for p in _fields_iterator(kluctl_fields['fieldsV1'], []))

    did_copy = False
    for mf in v1_fields:
        if mf['manager'] in overwrite_allowed_managers:
            continue
        for p in _fields_iterator(mf['fieldsV1'], []):
            if not p or [-1] == '.':
                continue
            if tuple(p) in ignored_fields:
                continue
            if tuple(p) in kluctl_fields:
                continue

            d, k, found = nav_managed_field(o, p)
            if found:
                if not did_copy:
                    did_copy = True
                    o = copy_dict(o)
                    d, k, found = nav_managed_field(o, p)
                    assert found
                del d[k]

    return o


def remove_non_managed_fields2(o, remote_objects):
    r = remote_objects.get(get_object_ref(o))
    if r is not None:
        mf = r['metadata'].get('managedFields')
        if mf is None:
            return o
        else:
            return remove_non_managed_fields(o, mf)
    else:
        return o
