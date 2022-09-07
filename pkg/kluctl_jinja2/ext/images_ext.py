import base64
import json

from jinja2.ext import Extension

begin_placeholder = "XXXXXbegin_get_image_"
end_placeholder = "_end_get_imageXXXXX"

class ImagesExtension(Extension):
    def __init__(self, environment):
        super().__init__(environment)
        environment.globals.update(self.build_images_vars())

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
