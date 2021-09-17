import fnmatch
import threading
from uuid import uuid4

from jsonpath_ng import parse, JSONPath

_dummy = str(uuid4())

json_path_cache = {}
json_path_cache_mutex = threading.Lock()

def parse_json_path(p) -> JSONPath:
    with json_path_cache_mutex:
        if p in json_path_cache:
            return json_path_cache[p]
        pp = parse(p)
        json_path_cache[p] = pp
        return pp

def del_if_exists(d, k):
    p = parse_json_path(k)
    p.filter(lambda x: True, d)

def set_if_not_exists(d, k, v):
    p = parse_json_path(k)
    f = p.find(d)
    if not f:
        p.update_or_create(d, v)

def del_if_falsy(d, k):
    p = parse_json_path(k)
    p.filter(lambda x: not x, d)

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
                stack.append((v, p + [str(i)]))

def del_matching_path(o, path):
    p = parse_json_path(path)
    p.filter(lambda x: True, o)
