import fnmatch
from uuid import uuid4

_dummy = str(uuid4())

def nav_dict(d, k):
    if isinstance(k, str):
        if "\\." in k:
            k = k.replace("\\.", _dummy)
            k = k.split(".")
            k = [x.replace(_dummy, ".") for x in k]
        else:
            k = k.split(".")
    elif not isinstance(k, list):
        raise ValueError("k must be a list and not %s" % type(k).__name__)

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
    for _, p in list(object_iterator(o)):
        if fnmatch.fnmatch(".".join(p), path):
            p2 = [x.replace(".", "\\.") for x in p]
            del_if_exists(o, ".".join(p2))
