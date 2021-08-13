import logging

from kluctl.utils.external_tools import get_external_tool_path
from kluctl.utils.utils import runHelper

logger = logging.getLogger('kustomize')

def kustomize(args, pathToYaml, ignoreErrors=False):
    args = [get_external_tool_path("kustomize")] + args

    r, out, err = runHelper(args, cwd=pathToYaml, logger=logger, stdout_log_level=logging.DEBUG - 1)
    if r != 0 and not ignoreErrors:
        raise Exception("kustomize failed: r=%d\nout=%s\nerr=%s" % (r, out, err))
    return r, str(out.decode("utf-8")), err

def kustomize_build(pathToYaml):
    r, out, err = kustomize(["build"], pathToYaml)
    return out
