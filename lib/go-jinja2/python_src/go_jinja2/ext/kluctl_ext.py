import base64
import hashlib
import json
import os
import textwrap

import sys

import jinja2
from jinja2 import TemplateError, TemplateNotFound
from jinja2.ext import Extension
from jinja2.runtime import Context

from .dict_utils import get_dict_value, merge_dict
from .yaml_utils import yaml_dump, yaml_load


class KluctlExtension(Extension):
    def __init__(self, environment):
        super().__init__(environment)
        add_jinja2_filters(environment)


def b64encode(string):
    return base64.b64encode(string.encode()).decode()


def b64decode(string):
    return base64.b64decode(string.encode()).decode()


def to_yaml(obj):
    return yaml_dump(obj)


def to_json(obj):
    return json.dumps(obj)


def from_yaml(s):
    return yaml_load(s)


@jinja2.pass_context
def render(ctx, string):
    # TODO we should deprecate render being available as filter, as filters seem to not have access to local variables
    t = ctx.environment.from_string(string)
    return t.render(ctx.get_all())


def sha256(s, digest_len=None):
    if not isinstance(s, bytes):
        s = s.encode("utf-8")
    hash = hashlib.sha256(s).hexdigest()
    if digest_len is not None:
        hash = hash[:digest_len]
    return hash


def slugify(s, **slugify_args):
    from slugify import slugify as _slugify
    return _slugify(s, **slugify_args)


@jinja2.pass_context
def load_template(ctx, path, **kwargs):
    ctx.environment.print_debug("load_template(%s)" % path)
    t = ctx.environment.get_template(path.replace(os.path.sep, '/'), parent=ctx.name)
    vars = merge_dict(ctx.get_all(), kwargs)
    return t.render(vars)


@jinja2.pass_context
def load_base64(ctx, path, width=None):
    ctx.environment.print_debug("load_base64(%s)" % path)
    p = path.replace(os.path.sep, '/')
    if ctx.name:
        p = ctx.environment.join_path(path, ctx.name)

    p = ctx.environment.loader._find_path(path)
    if not p:
        raise TemplateNotFound(path)

    contents, _, _ = ctx.environment.loader.read_template_helper(ctx.environment, p, True)

    contents = base64.b64encode(contents).decode("utf-8")
    if width:
        contents = textwrap.fill(contents, width=width)

    return contents


class VarNotFoundException(Exception):
    pass


@jinja2.pass_context
def get_var(ctx, path, default=None):
    if not isinstance(path, list):
        path = [path]

    all_vars = ctx.get_all()
    for p in path:
        r = get_dict_value(all_vars, p, VarNotFoundException())
        if isinstance(r, VarNotFoundException):
            continue
        return r
    return default


def update_dict(a, b):
    merge_dict(a, b, False)
    return ""


def raise_helper(msg):
    raise TemplateError(msg)


def debug_print(msg):
    sys.stderr.write("debug_print: %s\n" % str(msg))
    return ""


@jinja2.pass_context
def load_sha256(ctx: Context, path, digest_len=None):
    if "__calc_sha256__" in ctx:
        return "__self_sha256__"
    rendered = load_template(ctx, path, __calc_sha256__=True)
    hash = hashlib.sha256(rendered.encode("utf-8")).hexdigest()
    if digest_len is not None:
        hash = hash[:digest_len]
    return hash


def add_jinja2_filters(jinja2_env):
    jinja2_env.filters['b64encode'] = b64encode
    jinja2_env.filters['b64decode'] = b64decode
    jinja2_env.filters['to_yaml'] = to_yaml
    jinja2_env.filters['to_json'] = to_json
    jinja2_env.filters['from_yaml'] = from_yaml
    # render is available as filter and as global function. The filter should be deprecated in the future as it is
    # unable to access local variables
    jinja2_env.filters['render'] = render
    jinja2_env.filters['sha256'] = sha256
    jinja2_env.filters['slugify'] = slugify
    jinja2_env.globals['load_template'] = load_template
    jinja2_env.globals['load_base64'] = load_base64
    jinja2_env.globals['get_var'] = get_var
    jinja2_env.globals['merge_dict'] = merge_dict
    jinja2_env.globals['update_dict'] = update_dict
    jinja2_env.globals['raise'] = raise_helper
    jinja2_env.globals['debug_print'] = debug_print
    jinja2_env.globals['load_sha256'] = load_sha256
    jinja2_env.globals['render'] = render
