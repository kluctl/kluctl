import base64
import json
from concurrent.futures import ThreadPoolExecutor

from jinja2 import StrictUndefined, FileSystemLoader

import jinja2_server_pb2
import jinja2_server_pb2_grpc
from dict_utils import merge_dict
from jinja2_cache import KluctlBytecodeCache
from jinja2_utils import KluctlJinja2Environment, add_jinja2_filters, extract_template_error
from yaml_utils import yaml_load

jinja2_cache = KluctlBytecodeCache(max_cache_files=10000)

begin_placeholder = "XXXXXbegin_get_image_"
end_placeholder = "_end_get_imageXXXXX"

class Jinja2Servicer(jinja2_server_pb2_grpc.Jinja2ServerServicer):
    def __init__(self):
        self.executor = ThreadPoolExecutor()

    def get_image_wrapper(self, image, latest_version=None):
        if latest_version is None:
            latest_version = "semver()"
        placeholder = {
            "image": image,
            "latestVersion": str(latest_version),
        }
        j = json.dumps(placeholder)
        j = base64.b64encode(j.encode("utf8")).decode("utf8")
        j = begin_placeholder + j + end_placeholder
        return j

    def build_images_vars(self):
        def semver(allow_no_nums=False):
            return "semver(allow_no_nums=%s)" % allow_no_nums
        def prefix(s, suffix=None):
            if suffix is None:
                return "prefix(\"%s\")" % s
            else:
                return "prefix(\"%s\", suffix=\"%s\")" % (s, suffix)
        def number():
            return "number()"
        def regex(r):
            return "regex(\"%s\")" % r

        vars = {
            'images': {
                'get_image': self.get_image_wrapper,
            },
            'version': {
                'semver': semver,
                'prefix': prefix,
                'number': number,
                'regex': regex,
            },
        }
        return vars

    def build_env(self, vars_str, search_dirs):
        vars = json.loads(vars_str)
        image_vars = self.build_images_vars()
        merge_dict(vars, image_vars, clone=False)

        environment = KluctlJinja2Environment(loader=FileSystemLoader(search_dirs), undefined=StrictUndefined,
                                              cache_size=10000,
                                              bytecode_cache=jinja2_cache, auto_reload=False)
        merge_dict(environment.globals, vars, clone=False)

        add_jinja2_filters(environment)
        return environment

    def prepare_result(self, cnt):
        result = jinja2_server_pb2.JobResult()
        for i in range(cnt):
            r = jinja2_server_pb2.SingleResult()
            result.results.append(r)
        return result


    def render_helper(self, request, is_string):
        result = self.prepare_result(len(request.templates))
        env = self.build_env(request.vars, request.search_dirs)

        def do_render(i, t):
            try:
                if is_string:
                    t = env.from_string(t)
                else:
                    t = env.get_template(t)
                result.results[i].result = t.render()
            except Exception as e:
                result.results[i].error = extract_template_error(e)

        futures = []
        for i, t in enumerate(request.templates):
            f = self.executor.submit(do_render, i, t)
            futures.append(f)

        for f in futures:
            f.result()
        return result

    def RenderStrings(self, request, context):
        return self.render_helper(request, True)

    def RenderFiles(self, request, context):
        return self.render_helper(request, False)
