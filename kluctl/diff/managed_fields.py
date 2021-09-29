import dataclasses
import json
from typing import Any

from kluctl.utils.dict_utils import copy_dict, object_iterator, del_dict_value, get_dict_value, is_iterable, \
    set_dict_value
from kluctl.utils.jsonpath_utils import convert_list_to_json_path

# We automatically force overwrite these fields as we assume these are human-edited
overwrite_allowed_managers = {
    "kluctl",
    "kubectl",
    "kubectl-edit",
    "kubectl-client-side-apply",
    "rancher",
}

def check_item_match(o, kv, index):
    if kv[0:2] == "k:":
        k = kv[2:]
        j = json.loads(k)
        for k2 in j.keys():
            if k2 not in o:
                return False
            if j[k2] != o[k2]:
                return False
        return True
    elif kv[0:2] == "v:":
        k = kv[2:]
        j = json.loads(k)
        return j == o
    elif kv[0:2] == "i:":
        index2 = int(kv[2:])
        return index == index2
    else:
        raise ValueError("Invalid managedFields entry '%s'" % kv)

def convert_to_json_path(o, mf_path):
    ret = []
    for i in range(len(mf_path)):
        k = mf_path[i]
        if k == "{}":
            raise Exception()
        if k == ".":
            if i != len(mf_path) - 1:
                raise ValueError("Unexpected . element at %s" % convert_list_to_json_path(ret))
            return ret, True
        if k[:2] == 'f:':
            k = k[2:]
            if not isinstance(o, dict):
                raise ValueError("%s is not a dict" % convert_list_to_json_path(ret))
            if k not in o:
                return ret, False
            ret.append(k)
            o = o[k]
        else:
            if not is_iterable(o, False):
                raise ValueError("%s is not a list" % convert_list_to_json_path(ret))
            found = False
            for j, v in enumerate(o):
                if check_item_match(v, k, j):
                    found = True
                    ret.append(j)
                    o = o[j]
                    break
            if not found:
                return ret, False
    return ret, True

not_found = object()

@dataclasses.dataclass
class OverwrittenField:
    path: str
    local_value: Any
    remote_value: Any
    value: Any
    field_manager: str

def resolve_field_manager_conflicts(local_object, remote_object):
    overwritten = []

    managed_fields = get_dict_value(remote_object, "metadata.managedFields")
    if managed_fields is None:
        return local_object, overwritten
    v1_fields = [mf for mf in managed_fields if mf['fieldsType'] == 'FieldsV1']

    local_field_owners = {}
    for mf in v1_fields:
        for v, p in object_iterator(mf["fieldsV1"], only_leafs=True):
            local_json_path, local_found = convert_to_json_path(local_object, p)

            if local_found:
                local_field_owners[tuple(local_json_path)] = mf

    def find_owner(owners, p):
        tp = tuple(p)
        fm = None
        while len(tp) > 0:
            fm = owners.get(tp)
            if fm is not None:
                break
            tp = tp[:-1]
        return fm, tp

    ret = copy_dict(local_object)
    to_delete = set()
    for v, p in object_iterator(local_object, only_leafs=True):
        remote_value = get_dict_value(remote_object, p, not_found)

        fm, tp = find_owner(local_field_owners, p)

        if fm is None:
            # No manager found that claimed this field. If it's not existing in the remote object, it means it's a
            # new field so we can safely claim it. If it's present in the remote object AND has changed, it's a system
            # field that we have no control over!
            if remote_value is not not_found:
                if v != remote_value:
                    set_dict_value(ret, p, remote_value)
                    overwritten.append(OverwrittenField(path=convert_list_to_json_path(p),
                                                        local_value=v if v is not not_found else None,
                                                        remote_value=remote_value,
                                                        value=remote_value, field_manager="<none>"))
        elif fm["manager"] not in overwrite_allowed_managers:
            to_delete.add(tp)
            if v != remote_value:
                overwritten.append(OverwrittenField(path=convert_list_to_json_path(p),
                                                    local_value=v if v is not not_found else None,
                                                    remote_value=remote_value if remote_value is not not_found else None,
                                                    value=None, field_manager=fm["manager"]))

    for p in to_delete:
        # We do not own this field, so we should also not set it (not even to the same value to ensure we don't
        # claim shared ownership)
        del_dict_value(ret, p)

    return ret, overwritten
