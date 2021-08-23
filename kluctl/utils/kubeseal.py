import logging
import os
import tempfile

from kluctl.utils.run_helper import run_helper
from kluctl.utils.utils import get_tmp_base_dir

logger = logging.getLogger('kubeseal')

def kubeseal_raw(cert_file, secret, name, namespace, scope):
    with tempfile.NamedTemporaryFile(dir=get_tmp_base_dir(), delete=False) as tmp:
        try:
            tmp.write(secret)
            tmp.close()
            return kubeseal_raw_path(cert_file, tmp.name, name, namespace, scope)
        finally:
            os.unlink(tmp.name)

def kubeseal_raw_path(cert_file, secret_path, name, namespace, scope):
    args = ['kubeseal', '-oyaml']
    args += ["--raw", "--cert", cert_file, "--from-file", secret_path]
    if name is not None:
        args += ["--name", name]
    if namespace is not None:
        args += ['--namespace', namespace]
    if scope is not None:
        args += ["--scope", scope]
    r, out, err = run_helper(args, print_stderr=True, return_std=True)
    if r != 0:
        raise Exception("kubeseal returned %d, " % (r))
    return out.decode("utf-8")

def kubeseal_fetch_cert(context, controller_ns, controller_name):
    args = ['kubeseal', '--fetch-cert']
    args += ['--context', context, '--controller-namespace', controller_ns, '--controller-name', controller_name]
    r, out, err = run_helper(args, print_stderr=True, return_std=True)
    if r != 0:
        raise Exception("kubeseal returned %d, " % (r))
    return out
