import io
import os
import posixpath
import traceback
import typing

import sys
from jinja2 import Environment, TemplateNotFound, BaseLoader
from jinja2.loaders import split_template_path


# This Jinja2 environment allows to load templates relative to the parent template. This means that for example
# '{% include "file.yml" %}' will try to include the template from a ./file.yml
class MyEnvironment(Environment):
    def __init__(self, debug_enabled, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.debug_enabled = debug_enabled

    """Override join_path() to enable relative template paths."""
    """See https://stackoverflow.com/a/3655911/7132642"""
    def join_path(self, template, parent):
        result = self._join_path(template, parent)
        self.print_debug("join_path(%s, %s) - result=%s" % (template, parent, result))
        return result

    def _join_path(self, template, parent):
        if template[:2] == "./" or template[:3] == "../":
            p = os.path.join(os.path.dirname(parent), template)
            p = os.path.normpath(p)
            p = p.replace(os.path.sep, '/')
            return p
        return template

    def print_debug(self, s):
        if self.debug_enabled:
            print(s, file=sys.stderr)


class MyLoader(BaseLoader):
    def __init__(self, searchpath):
        super().__init__()
        self.root_template = None
        self.searchpath = searchpath

    def get_source(
            self, environment: "MyEnvironment", template: str
    ) -> typing.Tuple[str, typing.Optional[str], typing.Optional[typing.Callable[[], bool]]]:
        return self.debug_wrap_get_source("MyLoader", self._get_source, environment, template)

    def _find_path(self, template):
        if template == self.root_template:
            return template

        p = self._find_path_abs_search_path(template)
        if p:
            return p

        p = self._find_path_rel_search_path(template)
        if p:
            return p

        return None

    def _find_path_abs_search_path(self, template):
        try:
            if not os.path.isabs(template):
                return None
            template = os.path.abspath(template)
            for s in self.searchpath:
                sabs = os.path.abspath(s)
                if template.startswith(sabs + os.path.sep):
                    return template
            return None
        except OSError:
            return None

    def _find_path_rel_search_path(self, template):
        pieces = split_template_path(template)
        for searchpath in self.searchpath:
            # Use posixpath even on Windows to avoid "drive:" or UNC
            # segments breaking out of the search directory.
            filename = posixpath.join(searchpath, *pieces)
            if os.path.isfile(filename):
                return filename
        return None

    def _get_source(
            self, environment: "MyEnvironment", template: str
    ) -> typing.Tuple[str, typing.Optional[str], typing.Optional[typing.Callable[[], bool]]]:
        p = self._find_path(template)
        if not p:
            raise TemplateNotFound(template)
        return self.read_template_helper(environment, p)

    def read_template_helper(self, environment, template, binary=False):
        environment.print_debug("_read_template_helper %s" % template)
        try:
            mode = "r"
            if binary:
                mode += "b"
            with open(template, mode) as f:
                contents = f.read()
        except OSError:
            raise TemplateNotFound(template)

        mtime = os.path.getmtime(template)

        def uptodate() -> bool:
            try:
                return os.path.getmtime(template) == mtime
            except OSError:
                return False

        return contents, os.path.normpath(template), uptodate

    def debug_wrap_get_source(self, prefix, fn, environment, template):
        if not environment.debug_enabled:
            return fn(environment, template)
        try:
            contents, filename, uptodate = fn(environment, template)
            print("%s.get_source(template=%s), result=found" % (prefix, template), file=sys.stderr)
            return contents, filename, uptodate
        except TemplateNotFound:
            print("%s.get_source(template=%s), result=TemplateNotFound" % (prefix, template), file=sys.stderr)
            raise


def extract_template_error(e):
    try:
        raise e
    except TemplateNotFound as e2:
        return "template %s not found" % str(e2)
    except:
        etype, value, tb = sys.exc_info()
    extracted_tb = traceback.extract_tb(tb)
    found_template = None
    for i, s in reversed(list(enumerate(extracted_tb))):
        if not s.filename.endswith(".py"):
            found_template = i
            break
    f = io.StringIO()
    if found_template is not None:
        traceback.print_list([extracted_tb[found_template]], file=f)
        print("%s: %s" % (type(e).__name__, str(e)), file=f)
    else:
        traceback.print_exception(etype, value, tb, file=f)
    return f.getvalue()
