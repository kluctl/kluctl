# Add better wildcard support to jsonpath lib
import fnmatch
from jsonpath_ng import auto_id_field, jsonpath, JSONPath
from jsonpath_ng.exceptions import JsonPathParserError
from jsonpath_ng.ext.parser import ExtendedJsonPathParser


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

json_path_cache = {}
json_path_parser = ExtendedJsonPathParser()

def parse_json_path(p) -> JSONPath:
    if isinstance(p, list) or isinstance(p, tuple):
        p = convert_list_to_json_path(p)

    if p in json_path_cache:
        return json_path_cache[p]

    try:
        pp = json_path_parser.parse(p)
    except JsonPathParserError as e:
        raise Exception("Invalid json path '%s'. Error=%s" % (p, str(e)))

    pp = json_path_cache.setdefault(p, pp)

    return pp
