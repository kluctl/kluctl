import os
import re


def parse_env_config_sets(prefix):
    r = re.compile(r"%s_(\d+)_(.*)" % prefix)
    r2 = re.compile(r"%s_(.*)" % prefix)

    ret = {}
    for env_name, env_value in os.environ.items():
        m = r.match(env_name)
        if m:
            idx = m.group(1)
            key = m.group(2)
            ret.setdefault(idx, {})[key] = env_value
        else:
            m = r2.match(env_name)
            if m:
                key = m.group(1)
                ret.setdefault(None, {})[key] = env_value

    return ret
