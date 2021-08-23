from kluctl.utils.external_tools import get_external_tool_path
from kluctl.utils.run_helper import run_helper


def kustomize(args, pathToYaml, ignoreErrors=False):
    args = [get_external_tool_path("kustomize")] + args

    r, out, err = run_helper(args, cwd=pathToYaml, print_stderr=True, return_std=True)
    if r != 0 and not ignoreErrors:
        raise Exception("kustomize failed: r=%d" % r)
    return r, str(out.decode("utf-8")), err

def kustomize_build(pathToYaml):
    r, out, err = kustomize(["build", "--reorder", "none"], pathToYaml)
    return out
