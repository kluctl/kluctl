# Add better wildcard support to jsonpath lib
import fnmatch
import logging
import os
import threading

import ply
from jsonpath_ng import auto_id_field, jsonpath, JSONPath
from jsonpath_ng.exceptions import JsonPathParserError
from jsonpath_ng.ext.parser import ExtentedJsonPathParser
from jsonpath_ng.parser import IteratorToTokenStream


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

# This class pre-creates the yacc parser to speed up things
class MyJsonPathParser(ExtentedJsonPathParser):
    '''
    Dummy doc-string to avoid exceptions
    '''

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

        start_symbol = 'jsonpath'

        output_directory = os.path.dirname(__file__)
        try:
            module_name = os.path.splitext(os.path.split(__file__)[1])[0]
        except:
            module_name = __name__

        parsing_table_module = '_'.join([module_name, start_symbol, 'parsetab'])
        self.parser = ply.yacc.yacc(module=self,
                                   debug=self.debug,
                                   tabmodule=parsing_table_module,
                                   outputdir=output_directory,
                                   write_tables=0,
                                   start=start_symbol,
                                   errorlog=logger)

    def parse_token_stream(self, token_iterator, start_symbol='jsonpath'):
        assert start_symbol == "jsonpath"
        return self.parser.parse(lexer = IteratorToTokenStream(token_iterator))


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
json_path_local = threading.local()

def parse_json_path(p) -> JSONPath:
    if isinstance(p, list) or isinstance(p, tuple):
        p = convert_list_to_json_path(p)

    if p in json_path_cache:
        return json_path_cache[p]

    try:
        if not hasattr(json_path_local, "parser"):
            json_path_local.parser = MyJsonPathParser()

        pp = json_path_local.parser.parse(p)
    except JsonPathParserError as e:
        raise Exception("Invalid json path '%s'. Error=%s" % (p, str(e)))
    pp = json_path_cache.setdefault(p, pp)

    return pp
