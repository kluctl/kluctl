import re

from kluctl.command_error import CommandError
from kluctl.utils.utils import merge_dict


def parse_args(args_list):
    r = re.compile('^[a-zA-Z0-9\-_./]*=.*$')
    args = {}
    for arg in args_list:
        if not r.match(arg):
            raise CommandError("Invalid --arg argument. Must be --arg=some_var_name=value")
        s = arg.split("=", 1)
        name = s[0]
        value = s[1]
        args[name] = value
    return args

def check_required_args(args_def, args):
    defaults = {}
    for a in args_def:
        if 'default' not in a:
            continue
        name = a['name'].split('.')
        m = defaults
        for i, n in enumerate(name):
            if i == len(name) - 1:
                m[n] = a['default']
            else:
                if n not in m:
                    m[n] = {}
                m = m[n]

    for a in args_def:
        name = a['name'].split('.')
        m = args
        for i, n in enumerate(name):
            if n not in m:
                if 'default' not in a:
                    raise CommandError('Required argument %s not set' % a['name'])
                break
            m = m[n]
    args = merge_dict(defaults, args)
    return args
