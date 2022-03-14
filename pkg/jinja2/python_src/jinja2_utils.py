import base64
import hashlib
import io
import json
import logging
import os
import sys
import traceback

import jinja2
from jinja2 import Environment, TemplateNotFound, TemplateError
from jinja2.runtime import Context

from dict_utils import merge_dict, get_dict_value
from yaml_utils import yaml_dump, yaml_load

logger = logging.getLogger(__name__)

# This Jinja2 environment allows to load templates relative to the parent template. This means that for example
# '{% include "file.yml" %}' will try to include the template from a ./file.yml
class KluctlJinja2Environment(Environment):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.add_extension("jinja2.ext.loopcontrols")

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

@jinja2.pass_context
def get_var(ctx, path, default):
    if not isinstance(path, list):
        path = [path]
    for p in path:
        r = get_dict_value(ctx.parent, p, VarNotFoundException())
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
    logger.info("debug_print: %s" % str(msg))
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
    jinja2_env.filters['render'] = render
    jinja2_env.filters['sha256'] = sha256
    jinja2_env.globals['load_template'] = load_template
    jinja2_env.globals['get_var'] = get_var
    jinja2_env.globals['merge_dict'] = merge_dict
    jinja2_env.globals['update_dict'] = update_dict
    jinja2_env.globals['raise'] = raise_helper
    jinja2_env.globals['debug_print'] = debug_print
    jinja2_env.globals['load_sha256'] = load_sha256

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
