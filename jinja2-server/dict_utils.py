from jsonpath_utils import parse_json_path


def copy_primitive_value(v):
    if isinstance(v, dict):
        return copy_dict(v)
    if is_iterable(v, False):
        return [copy_primitive_value(x) for x in v]
    return v

def copy_dict(a):
    ret = {}
    for k, v in a.items():
        ret[k] = copy_primitive_value(v)
    return ret

def merge_dict(a, b, clone=True):
    if clone:
        a = copy_dict(a)
    if a is None:
        a = {}
    if b is None:
        b = {}
    for key in b:
        if key in a:
            if isinstance(a[key], dict) and isinstance(b[key], dict):
                merge_dict(a[key], b[key], clone=False)
            else:
                a[key] = b[key]
        else:
            a[key] = b[key]
    return a


def get_dict_value(y, path, default=None):
    p = parse_json_path(path)
    f = p.find(y)
    if len(f) > 1:
        raise Exception("Only simple jsonpath supported in get_dict_value")
    if len(f) == 0:
        return default
    return f[0].value

def is_iterable(obj, str_and_bytes=True):
    if isinstance(obj, list):
        return True
    if isinstance(obj, tuple):
        return True
    if isinstance(obj, dict):
        return True
    if isinstance(obj, str):
        return str_and_bytes
    if isinstance(obj, bytes):
        return str_and_bytes
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
