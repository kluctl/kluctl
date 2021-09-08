from kluctl.utils.dict_nav_utils import nav_dict, is_iterable


def copy_primitive_value(v):
    if isinstance(v, dict):
        return copy_dict(v)
    if isinstance(v, str) or isinstance(v, bytes):
        return v
    if is_iterable(v):
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

def set_default_value(d, n, default):
    if n not in d or d[n] is None:
        d[n] = default

def get_dict_value(y, path, default=None):
    d, k, found = nav_dict(y, path)
    if not found:
        return default
    return d[k]

def set_dict_value(y, path, value, do_clone=False):
    if do_clone:
        y = copy_dict(y)
    d, k, found = nav_dict(y, path)
    d[k] = value
    return y

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
