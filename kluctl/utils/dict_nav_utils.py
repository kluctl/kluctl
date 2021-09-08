import fnmatch

from kluctl.utils.dict_utils import is_iterable


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

def object_path_iterator(o):
    return _object_path_iterator(o, [])

def del_matching_path(o, path):
    for p in list(object_path_iterator(o)):
        if fnmatch.fnmatch(".".join(p), path):
            p2 = [x.replace(".", "\\.") for x in p]
            del_if_exists(o, ".".join(p2))
