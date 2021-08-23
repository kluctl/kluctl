import base64
import hashlib
import json
import logging
import os
import sys
import traceback

import jinja2
from jinja2 import Environment, StrictUndefined, FileSystemLoader, TemplateNotFound, TemplateError

from kluctl.utils.dict_utils import merge_dict
from kluctl.utils.yaml_utils import yaml_dump, yaml_load

logger = logging.getLogger(__name__)

# This Jinja2 environment allows to load templaces relative to the parent template. This means that for example
# '{% include "file.yml" %}' will try to include the template from a ./file.yml
class RelEnvironment(Environment):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)

    """Override join_path() to enable relative template paths."""
    """See https://stackoverflow.com/a/3655911/7132642"""
    def join_path(self, template, parent):
        p = os.path.join(os.path.dirname(parent), template)
        p = os.path.normpath(p)
        return p.replace('\\', '/')

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
    t = ctx.environment.from_string(string)
    return t.render(ctx.parent)

def sha256(s):
    if not isinstance(s, bytes):
        s = s.encode("utf-8")
    return hashlib.sha256(s).hexdigest()

@jinja2.pass_context
def load_template(ctx, path, **kwargs):
    dir = os.path.dirname(ctx.name)
    full_path = os.path.join(dir, path)
    try:
        t = ctx.environment.get_template(full_path.replace('\\', '/'))
    except TemplateNotFound:
        t = ctx.environment.get_template(path.replace('\\', '/'))
    vars = merge_dict(ctx.parent, kwargs)
    return t.render(vars)

class VarNotFoundException(Exception):
    pass

def get_var2(ctx, path):
    path = path.split('.')
    v = ctx.parent
    for p in path:
        if p not in v:
            raise VarNotFoundException()
        v = v.get(p)
    return v

@jinja2.pass_context
def get_var(ctx, path, default):
    if not isinstance(path, list):
        path = [path]
    for p in path:
        try:
            r = get_var2(ctx, p)
            return r
        except VarNotFoundException:
            pass
    return default

def update_dict(a, b):
    merge_dict(a, b, False)
    return ""

def raise_helper(msg):
    raise Exception(msg)

def debug_print(msg):
    logger.info("debug_print: %s" % str(msg))
    return ""

def add_jinja2_filters(jinja2_env):
    jinja2_env.filters['b64encode'] = b64encode
    jinja2_env.filters['b64decode'] = b64decode
    jinja2_env.filters['to_yaml'] = to_yaml
    jinja2_env.filters['to_json'] = to_json
    jinja2_env.filters['from_yaml'] = from_yaml
    jinja2_env.filters['render'] = render
    jinja2_env.filters['sha256'] = sha256
    jinja2_env.globals['load_template'] = load_template
    jinja2_env.globals['get_var'] = get_var
    jinja2_env.globals['merge_dict'] = merge_dict
    jinja2_env.globals['update_dict'] = update_dict
    jinja2_env.globals['raise'] = raise_helper
    jinja2_env.globals['debug_print'] = debug_print

def render_str(s, jinja_vars):
    if "{" not in s:
        return s
    e = RelEnvironment(undefined=StrictUndefined)
    add_jinja2_filters(e)
    merge_dict(e.globals, jinja_vars, False)
    t = e.from_string(s)
    return t.render()

def render_dict_strs2(d, jinja_vars, errors):
    if isinstance(d, dict):
        ret = {}
        for n, v in d.items():
            ret[n] = render_dict_strs2(v, jinja_vars, errors)
        return ret
    elif isinstance(d, list):
        ret = []
        for v in d:
            ret.append(render_dict_strs2(v, jinja_vars, errors))
        return ret
    elif isinstance(d, str):
        try:
            return render_str(d, jinja_vars)
        except TemplateError as e:
            errors.append(e)
            return d
    else:
        return d

def render_dict_strs(d, jinja_vars, do_raise=True):
    errors = []
    ret = render_dict_strs2(d, jinja_vars, errors)
    if do_raise:
        if errors:
            raise errors[0]
        return ret
    return ret, errors

def render_file(root_dir, path, jinja_vars):
    e = RelEnvironment(loader=FileSystemLoader(root_dir), undefined=StrictUndefined)
    path = os.path.normpath(path)
    add_jinja2_filters(e)
    merge_dict(e.globals, jinja_vars, False)
    t = e.get_template(path.replace('\\', '/'))
    return t.render()

def print_template_error(e):
    try:
        raise e
    except:
        etype, value, tb = sys.exc_info()
    extracted_tb = traceback.extract_tb(tb)
    found_template = None
    for i, s in reversed(list(enumerate(extracted_tb))):
        if not s.filename.endswith(".py"):
            found_template = i
            break
    if found_template is not None:
        traceback.print_list([extracted_tb[found_template]])
        print("%s: %s" % (type(e).__name__, str(e)), file=sys.stderr)
    else:
        traceback.print_exception(etype, value, tb)
