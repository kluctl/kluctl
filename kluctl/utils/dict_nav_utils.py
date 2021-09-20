import fnmatch
import threading
from uuid import uuid4

from jsonpath_ng import JSONPath, jsonpath, auto_id_field
from jsonpath_ng.ext import parse

_dummy = str(uuid4())

json_path_cache = {}
json_path_cache_mutex = threading.Lock()


def ext_reified_fields(self, datum):
    result = []
    for field in self.fields:
        if "*" not in field:
            result.append(field)
            continue
        try:
            fields = [f for f in datum.value.keys() if fnmatch.fnmatch(f, field)]
            if auto_id_field is not None:
                fields.append(auto_id_field)
            result += fields
        except AttributeError:
            pass
    return tuple(result)
jsonpath.Fields.reified_fields = ext_reified_fields

def parse_json_path(p) -> JSONPath:
    if isinstance(p, list):
        p2 = "$"
        for x in p:
            if isinstance(x, str):
                if '"' in x:
                    p2 = "%s['%s']" % (p2, x)
                else:
                    p2 = '%s["%s"]' % (p2, x)
            else:
                p2 = "%s[%d]" % (p2, x)
        p = p2

    with json_path_cache_mutex:
        if p in json_path_cache:
            return json_path_cache[p]
        pp = parse(p)
        json_path_cache[p] = pp
        return pp


def is_iterable(obj):
    if isinstance(obj, list):
        return True
    if isinstance(obj, tuple):
        return True
    if isinstance(obj, dict):
        return True
    if isinstance(obj, str):
        return True
    if isinstance(obj, bytes):
        return True
    if isinstance(obj, int) or isinstance(obj, bool):
        return False
    if isinstance(obj, type):
        return False
    try:
        iter(obj)
    except Exception:
        return False
    else:
        return True

def object_iterator(o):
    stack = [(o, [])]

    while len(stack) != 0:
        o2, p = stack.pop()
        yield o2, p

        if isinstance(o2, dict):
            for k, v in o2.items():
                stack.append((v, p + [k]))
        elif not isinstance(o2, str) and is_iterable(o2):
            for i, v in enumerate(o2):
                stack.append((v, p + [i]))
