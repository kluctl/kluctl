import dataclasses
import difflib
import re

from deepdiff import DeepDiff
from deepdiff.helper import NotPresent

from kluctl.utils.k8s_object_utils import get_object_ref
from kluctl.utils.dict_utils import copy_dict
from kluctl.utils.yaml_utils import yaml_dump


def is_empty(o):
    if isinstance(o, dict) or isinstance(o, list):
        return len(o) == 0
    return False

def remove_empty(o):
    if isinstance(o, dict):
        for k in list(o.keys()):
            remove_empty(o[k])
            if is_empty(o[k]):
                del(o[k])
    elif isinstance(o, list):
        i = 0
        while i < len(o):
            v = o[i]
            remove_empty(v)
            if is_empty(v):
                o.pop(i)
            else:
                i += 1

def object_to_diffable_lines(o):
    if o is None:
        return ['null']
    if isinstance(o, NotPresent):
        return []
    if isinstance(o, str):
        return o.splitlines()
    if isinstance(o, bool):
        return ['true' if o else 'false']
    if isinstance(o, int):
        return [str(o)]
    y = yaml_dump(o)
    return y.splitlines()

def unified_diff_object(old_object, new_object):
    old_lines = object_to_diffable_lines(old_object)
    new_lines = object_to_diffable_lines(new_object)

    if len(old_lines) == 0:
        d = ['+' + x for x in new_lines]
    elif len(new_lines) == 0:
        d = ['-' + x for x in old_lines]
    elif len(old_lines) == 1 and len(new_lines) == 1:
        d = ['-%s' % old_lines[0], '+%s' % new_lines[0]]
    else:
        d = list(difflib.unified_diff(old_lines, new_lines))
        # Skip diff header
        d = d[2:]
    d = '\n'.join(d)
    return d

diff_path_regex = re.compile(r'\[([^\]]*)\]')

def to_dotted_path(diff_path):
    diff_path = diff_path[len('root'):]
    s = diff_path_regex.split(diff_path)
    s = [l for l in s if l]

    def remove_quotes(s):
        return s[1:-1] if s.startswith('\'') else s
    def escape(s):
        return s.replace('.', '\\.')

    for i in range(len(s)):
        s[i] = remove_quotes(s[i])
        s[i] = escape(s[i])

    return '.'.join(s)

def eval_diff_path(diff_path, o):
    try:
        return eval(diff_path, {'root': o})
    except KeyError:
        return NotPresent()
    except:
        return '...'

def deep_diff_object(old_object, new_object, ignore_order):
    old_object = copy_dict(old_object)
    new_object = copy_dict(new_object)
    remove_empty(old_object)
    remove_empty(new_object)
    diff = DeepDiff(old_object, new_object, ignore_order=ignore_order)
    return deep_diff_to_changes(diff, old_object, new_object)

def deep_diff_to_changes(diff, old_object, new_object):
    changes = []
    for report_type in diff.keys():
        r = diff[report_type]
        for path in list(r):
            change = {
                "path": to_dotted_path(path)
            }
            if report_type in ['values_changed', 'type_changes']:
                e = r[path]
                change["old_value"] = e['old_value']
                change["new_value"] = e['new_value']
                change["unified_diff"] = unified_diff_object(e['old_value'], e['new_value'])
            elif report_type in ['dictionary_item_added', 'iterable_item_added', 'attribute_added']:
                change["new_value"] = eval_diff_path(path, new_object)
            elif report_type in ['dictionary_item_removed', 'iterable_item_removed', 'attribute_removed']:
                change["old_value"] = eval_diff_path(path, old_object)
            else:
                raise Exception('Unknown report_type=\'%s\'. items=%s' % (report_type, str(r)))

            changes.append(change)

    changes.sort(key=lambda x: x["path"])

    return changes

def changes_to_yaml(new_objects, changed_objects, errors, warnings):
    new_objects = [{
        "ref": dataclasses.asdict(get_object_ref(x)),
        "object": x
    } for x in new_objects]
    changed_objects = [{
        "ref": dataclasses.asdict(get_object_ref(c["old_object"])),
        "changes": c["changes"],
    } for c in changed_objects]
    errors = [dataclasses.asdict(x) for x in errors]
    warnings = [dataclasses.asdict(x) for x in warnings]

    return {
        "new": new_objects,
        "changed": changed_objects,
        "errors": errors,
        "warnings": warnings,
    }
