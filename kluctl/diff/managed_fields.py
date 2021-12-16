import json
import re

from kluctl.utils.dict_utils import copy_dict, object_iterator, del_dict_value, get_dict_value, is_iterable, \
    has_dict_value, list_matching_dict_pathes
from kluctl.utils.jsonpath_utils import convert_list_to_json_path, parse_json_path

# We automatically force overwrite these fields as we assume these are human-edited
from kluctl.utils.utils import parse_bool

FORCE_APPLY_FIELD_ANNOTATION_REGEX = re.compile(r"^kluctl.io/force-apply-field(-\d*)?$")

overwrite_allowed_managers = [
    "kluctl",
    "kubectl",
    "kubectl-.*",
    "rancher",
    "k9s",
]

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
            if o is None:
                return ret, False
            if not isinstance(o, dict):
                raise ValueError("%s is not a dict" % convert_list_to_json_path(ret))
            if k not in o:
                return ret, False
            ret.append(k)
            o = o[k]
        else:
            if o is None:
                return ret, False
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

def convert_to_string(mf_path):
    ret = []
    for i in range(len(mf_path)):
        k = mf_path[i]
        if k == "{}":
            raise Exception()
        if k == ".":
            if i != len(mf_path) - 1:
                raise ValueError("Unexpected . element at %s" % convert_list_to_json_path(ret))
            break
        if k[:2] == "f:":
            k = k[2:]
            ret.append(".%s" % k)
        elif k[:2] == "k:":
            strs = []
            k = json.loads(k[2:])
            for k2 in sorted(k.keys()):
                strs.append("%s=%s" % (k2, json.dumps(k[k2])))
            ret.append("[%s]" % ",".join(strs))
        elif k[:2] == "v:":
            ret.append("[=%s]" % k[2:])
        elif k[:2] == "i:":
            ret.append("[%s]" % k[2:])
        else:
            raise ValueError("Invalid path element %s" % k)
    return "".join(ret)

def resolve_field_manager_conflicts(local_object, remote_object, conflict_status):
    managed_fields = get_dict_value(remote_object, "metadata.managedFields")
    if managed_fields is None:
        raise Exception("managedFields not found")
    v1_fields = [mf for mf in managed_fields if mf['fieldsType'] == 'FieldsV1']

    # "stupid" because the string representation in "details.causes.field" might be ambiguous as k8s does not escape dots
    fields_as_stupid_strings = {}
    managers_by_field = {}
    for mf in v1_fields:
        for v, p in object_iterator(mf["fieldsV1"], only_leafs=True):
            s = convert_to_string(p)
            fields_as_stupid_strings.setdefault(s, set()).add(tuple(p))
            managers_by_field.setdefault(tuple(p), []).append(mf)

    ret = copy_dict(local_object)

    force_apply_all = get_dict_value(local_object, 'metadata.annotations["kluctl.io/force-apply"]', "false")
    force_apply_all = parse_bool(force_apply_all)

    force_apply_fields = set()
    for k, v in get_dict_value(local_object, "metadata.annotations", {}).items():
        if FORCE_APPLY_FIELD_ANNOTATION_REGEX.fullmatch(k):
            for x in list_matching_dict_pathes(ret, v):
                force_apply_fields.add(x)

    lost_ownership = []
    for cause in get_dict_value(conflict_status, "details.causes", []):
        if cause.get("reason") != "FieldManagerConflict" or "field" not in cause:
            raise Exception("Unknown reason '%s'" % cause.get("reason"))

        mf_path = fields_as_stupid_strings.get(cause["field"])
        if not mf_path:
            raise Exception("Could not find matching field for path '%s'" % cause["field"])
        if len(mf_path) != 1:
            raise Exception("field path '%s' is ambiguous" % cause["field"])
        mf_path = list(mf_path)[0]

        p, found = convert_to_json_path(remote_object, mf_path)
        if not found:
            raise Exception("Field '%s' not found in remote object" % cause["field"])
        if not has_dict_value(local_object, p):
            raise Exception("Field '%s' not found in local object" % cause["field"])

        local_value = get_dict_value(local_object, p)
        remote_value = get_dict_value(remote_object, p)

        overwrite = True
        if not force_apply_all:
            for mf in managers_by_field[mf_path]:
                if not any(re.fullmatch(x, mf["manager"]) for x in overwrite_allowed_managers):
                    overwrite = False
                    break
            if str(parse_json_path(p)) in force_apply_fields:
                overwrite = True

        if not overwrite:
            del_dict_value(ret, p)

            if local_value != remote_value:
                lost_ownership.append(cause)

    return ret, lost_ownership
