import contextlib
import hashlib
import os
import shutil

from kluctl.utils.run_helper import run_helper
from kluctl.utils.versions import LooseVersionComparator
from kluctl.utils.yaml_utils import yaml_load_file, yaml_load_all, yaml_dump_all, yaml_load, yaml_save_file


class HelmChart(object):
    def __init__(self, config_file):
        self.dir = os.path.dirname(config_file)
        self.config_file = config_file
        self.conf = yaml_load_file(config_file)['helmChart']

    def save(self, config_file):
        yaml_save_file({
            "helmChart": self.conf
        }, config_file)

    @contextlib.contextmanager
    def repo_context(self):
        need_repo = False
        repo_name = 'stable'
        if 'repo' in self.conf and self.conf['repo'] != 'stable':
            need_repo = True
            repo_name = "kluctl-%s" % hashlib.sha256(self.conf['repo'].encode("utf-8")).hexdigest()[:16]
        try:
            if need_repo:
                self.do_helm(['repo', 'remove', repo_name], ignoreErrors=True, ignoreStderr=True)
                self.do_helm(['repo', 'add', repo_name, self.conf['repo']])
            else:
                self.do_helm(['repo', 'update'])
            yield repo_name
        finally:
            if need_repo:
                self.do_helm(['repo', 'remove', repo_name], ignoreErrors=True, ignoreStderr=True)

    def pull(self):
        target_dir = os.path.join(self.dir, 'charts')

        with self.repo_context() as repo_name:
            rm_dir = os.path.join(target_dir, self.conf['chartName'])
            shutil.rmtree(rm_dir, ignore_errors=True)

            args = ['pull', '%s/%s' % (repo_name, self.conf['chartName']), '--destination', target_dir, '--untar']
            if 'chartVersion' in self.conf:
                args += ['--version', self.conf['chartVersion']]
            self.do_helm(args)

    def check_update(self):
        with self.repo_context() as repo_name:
            chart_name = '%s/%s' % (repo_name, self.conf['chartName'])
            args = ['search', 'repo', chart_name, '-oyaml', '-l']
            r, stdout, stderr = self.do_helm(args)
            l = yaml_load(stdout)
            # ensure we didn't get partial matches
            l = [x for x in l if x["name"] == chart_name]
            if len(l) == 0:
                raise Exception("Helm chart %s not found in repository" % self.conf['chartName'])
            l.sort(key=lambda x: LooseVersionComparator(x["version"]))
            latest_chart = l[-1]
            latest_version = latest_chart["version"]
            if latest_version == self.conf["chartVersion"]:
                return None
            return latest_version

    def render(self):
        chart_dir = os.path.join(self.dir, 'charts', self.conf['chartName'])
        values_path = os.path.join(self.dir, 'helm-values.yml')
        output_path = os.path.join(self.dir, self.conf['output'])

        args = ['template', self.conf['releaseName'], chart_dir]

        namespace = self.conf.get("namespace", "default")

        if os.path.exists(values_path):
            args += ['-f', values_path]
        args += ['-n', namespace]
        if self.conf.get('skipCRDs', False):
            args += ['--skip-crds']
        else:
            args += ['--include-crds']

        args.append("--skip-tests")

        r, rendered, stderr = self.do_helm(args)
        rendered = rendered.decode('utf-8')

        parsed = yaml_load_all(rendered)
        for o in parsed:
            if o is None:
                continue
            # "helm install" will deploy resources to the given namespace automatically, but "helm template" does not
            # add the necessary namespace in the rendered resources
            o.setdefault("metadata", {}).setdefault("namespace", namespace)
        rendered = yaml_dump_all(parsed)

        with open(output_path, 'w') as f:
            f.write(rendered)

    def do_helm(self, args, input=None, ignoreErrors=False, ignoreStderr=False):
        args = ['helm'] + args

        r, stdout, stderr = run_helper(args=args, input=input, print_stdout=False, print_stderr=not ignoreStderr, return_std=True)
        if r != 0 and not ignoreErrors:
            raise Exception("helm failed: r=%d\nout=%s\nerr=%s" % (r, stdout.decode("utf-8"), stderr.decode("utf-8")))
        return r, stdout, stderr
