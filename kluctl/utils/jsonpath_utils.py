# Add better wildcard support to jsonpath lib
import atexit
import fnmatch
import logging
import os
import pickle
import threading

from jsonpath_ng import auto_id_field, jsonpath, JSONPath
from jsonpath_ng.exceptions import JsonPathParserError
from jsonpath_ng.ext import parse

from kluctl.utils.exceptions import CommandError
from kluctl.utils.utils import get_tmp_base_dir

logger = logging.getLogger(__name__)

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

def convert_list_to_json_path(p):
    p2 = ""
    for x in p:
        if isinstance(x, str):
            if x.isalnum():
                if p2 != "":
                    p2 += "."
                p2 += "%s" % x
            else:
                if p2 == "":
                    p2 = "$"
                if '"' in x:
                    p2 = "%s['%s']" % (p2, x)
                else:
                    p2 = '%s["%s"]' % (p2, x)
        else:
            if p2 == "":
                p2 = "$"
            p2 = "%s[%d]" % (p2, x)
    return p2

json_path_cache = None
json_path_cache_modified = False
json_path_cache_lock = threading.Lock()

def load_json_path_cache():
    global json_path_cache
    if json_path_cache is not None:
        return
    with json_path_cache_lock:
        path = os.path.join(get_tmp_base_dir(), "kluctl-jsonpath-cache.bin")
        if not os.path.exists(path):
            json_path_cache = {}
            return
        try:
            with open(path, mode="rb") as f:
                json_path_cache = pickle.load(f)
        except Exception as e:
            logger.exception("Excpetion while loading jsonpath cache", exc_info=e)
            json_path_cache = {}

def save_json_path_cache():
    with json_path_cache_lock:
        if not json_path_cache:
            return
        if not json_path_cache_modified:
            return
        path = os.path.join(get_tmp_base_dir(), "kluctl-jsonpath-cache.bin")
        try:
            logger.debug("Storing %d jsonpath cache entries" % len(json_path_cache))
            with open(path + ".tmp", mode="wb") as f:
                pickle.dump(json_path_cache, f)
            os.rename(path + ".tmp", path)
        except Exception as e:
            logger.exception("Exception while storing jsonpath cache", exc_info=e)

atexit.register(save_json_path_cache)

def parse_json_path(p) -> JSONPath:
    if isinstance(p, list) or isinstance(p, tuple):
        p = convert_list_to_json_path(p)

    load_json_path_cache()

    if p in json_path_cache:
        return json_path_cache[p]

    try:
        pp = parse(p)
    except JsonPathParserError as e:
        raise CommandError("Invalid json path '%s'. Error=%s" % (p, str(e)))
    pp = json_path_cache.setdefault(p, pp)

    global json_path_cache_modified
    json_path_cache_modified = True

    return pp
