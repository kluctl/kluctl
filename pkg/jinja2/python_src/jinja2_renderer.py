import base64
import json

from jinja2 import StrictUndefined, FileSystemLoader, ChainableUndefined

from dict_utils import merge_dict
from jinja2_utils import KluctlJinja2Environment, add_jinja2_filters, extract_template_error


begin_placeholder = "XXXXXbegin_get_image_"
end_placeholder = "_end_get_imageXXXXX"


class NullUndefined(ChainableUndefined):
    def _return_self(self, other):
        return self

    __add__ = __radd__ = __sub__ = __rsub__ = _return_self
    __mul__ = __rmul__ = __div__ = __rdiv__ = _return_self
    __truediv__ = __rtruediv__ = _return_self
    __floordiv__ = __rfloordiv__ = _return_self
    __mod__ = __rmod__ = _return_self
    __pos__ = __neg__ = _return_self
    __call__ = __getitem__ = _return_self
    __lt__ = __le__ = __gt__ = __ge__ = _return_self
    __int__ = __float__ = __complex__ = _return_self
    __pow__ = __rpow__ = _return_self

class Jinja2Renderer:
    def __init__(self, trim_blocks=False, lstrip_blocks=False):
        self.trim_blocks = trim_blocks
        self.lstrip_blocks = lstrip_blocks

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

    def build_env(self, vars_str, search_dirs, strict):
        vars = json.loads(vars_str)
        image_vars = self.build_images_vars()
        merge_dict(vars, image_vars, clone=False)

        environment = KluctlJinja2Environment(loader=FileSystemLoader(search_dirs),
                                              undefined=StrictUndefined if strict else NullUndefined,
                                              cache_size=10000,
                                              auto_reload=False,
                                              trim_blocks=self.trim_blocks, lstrip_blocks=self.lstrip_blocks)
        merge_dict(environment.globals, vars, clone=False)

        add_jinja2_filters(environment)
        return environment

    def render_helper(self, templates, search_dirs, vars, is_string, strict):
        env = self.build_env(vars, search_dirs, strict)

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

    def RenderStrings(self, templates, search_dirs, vars, strict):
        try:
            return self.render_helper(templates, search_dirs, vars, True, strict)
        except Exception as e:
            return [{
                "error": str(e)
            }] * len(templates)

    def RenderFiles(self, templates, search_dirs, vars, strict):
        try:
            return self.render_helper(templates, search_dirs, vars, False, strict)
        except Exception as e:
            return [{
                "error": str(e)
            }] * len(templates)
