import sys

import yaml

try:
    from yaml import CSafeLoader as SafeLoader, CSafeDumper as SafeDumper
except ImportError:
    print("Failed to load fast LibYAML bindings. You should install them to speed up kluctl.", file=sys.stderr)
    from yaml import SafeLoader as SafeLoader, SafeDumper as SafeDumper


def construct_value(load, node):
    if not isinstance(node, yaml.ScalarNode):
        raise yaml.constructor.ConstructorError(
            "while constructing a value",
            node.start_mark,
            "expected a scalar, but found %s" % node.id, node.start_mark
        )
    yield str(node.value)


# See https://github.com/yaml/pyyaml/issues/89
SafeLoader.add_constructor(u'tag:yaml.org,2002:value', construct_value)


def multiline_str_representer(dumper, data):
    if len(data.splitlines()) > 1:  # check for multiline string
        return dumper.represent_scalar('tag:yaml.org,2002:str', data, style='|')
    return dumper.represent_scalar('tag:yaml.org,2002:str', data)

class MultilineStrDumper(SafeDumper):
    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.add_representer(str, multiline_str_representer)

def yaml_load(s):
    return yaml.load(s, Loader=SafeLoader)

def yaml_load_all(s):
    return list(yaml.load_all(s, Loader=SafeLoader))

def yaml_load_file(path, all=False):
    with open(path) as f:
        if all:
            y = yaml_load_all(f)
        else:
            y = yaml_load(f)
    return y

def yaml_dump(y, stream=None):
    return yaml.dump(y, stream=stream, Dumper=MultilineStrDumper, sort_keys=False)

def yaml_dump_all(y, stream=None):
    return yaml.dump_all(y, stream=stream, Dumper=MultilineStrDumper, sort_keys=False)

def yaml_save_file(y, path):
    with open(path, mode='w') as f:
        yaml_dump(y, f)
