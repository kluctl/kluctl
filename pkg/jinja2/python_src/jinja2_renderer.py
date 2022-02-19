import base64
import json

from jinja2 import StrictUndefined, FileSystemLoader

from dict_utils import merge_dict
from jinja2_cache import KluctlBytecodeCache
from jinja2_utils import KluctlJinja2Environment, add_jinja2_filters, extract_template_error

jinja2_cache = KluctlBytecodeCache(max_cache_files=10000)

begin_placeholder = "XXXXXbegin_get_image_"
end_placeholder = "_end_get_imageXXXXX"


class Jinja2Renderer:

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

    def render_helper(self, templates, search_dirs, vars, is_string):
        env = self.build_env(vars, search_dirs)

        result = []

        for i, t in enumerate(templates):
            try:
                if is_string:
                    t = env.from_string(t)
                else:
                    t = env.get_template(t)
                result.append({
                    "result": t.render()
                })
            except Exception as e:
                result.append({
                    "error": extract_template_error(e),
                })

        return result

    def RenderStrings(self, templates, search_dirs, vars):
        try:
            return self.render_helper(templates, search_dirs, vars, True)
        except Exception as e:
            return [{
                "error": str(e)
            }] * len(templates)

    def RenderFiles(self, templates, search_dirs, vars):
        try:
            return self.render_helper(templates, search_dirs, vars, False)
        except Exception as e:
            return [{
                "error": str(e)
            }] * len(templates)
