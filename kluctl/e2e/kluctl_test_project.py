import logging
import os
import shutil
import subprocess
import sys
from tempfile import TemporaryDirectory

from git import Git
from pytest_kind import KindCluster

from kluctl.utils.yaml_utils import yaml_save_file

logger = logging.getLogger(__name__)


class KluctlTestProject:
    def __init__(self, kluctl_project_external=False,
                 clusters_external=False, deployment_external=False, sealed_secrets_external=False,
                 local_clusters=None, local_deployment=None, local_sealed_secrets=None):
        self.kluctl_project_external = kluctl_project_external
        self.clusters_external = clusters_external
        self.deployment_external = deployment_external
        self.sealed_secrets_external = sealed_secrets_external
        self.local_clusters = local_clusters
        self.local_deployment = local_deployment
        self.local_sealed_secrets = local_sealed_secrets
        self.kubeconfigs = []

    def __enter__(self):
        self.base_dir = TemporaryDirectory()

        os.makedirs(self.get_kluctl_project_dir(), exist_ok=True)
        os.makedirs(self.get_clusters_dir(), exist_ok=True)
        os.makedirs(self.get_sealed_secrets_dir(), exist_ok=True)
        os.makedirs(self.get_deployment_dir(), exist_ok=True)

        self._git_init(self.get_kluctl_project_dir())
        if self.clusters_external:
            self._git_init(self.get_clusters_dir())
        if self.deployment_external:
            self._git_init(self.get_deployment_dir())
        if self.sealed_secrets_external:
            self._git_init(self.get_sealed_secrets_dir())

        #self._commit_dot_kluctl({"targets": []})

        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        return self.base_dir.__exit__(exc_type, exc_val, exc_tb)

    def _git_init(self, dir):
        os.makedirs(dir, exist_ok=True)
        g = Git(dir)
        g.init()

    def _commit(self, dir, add=None, all=None, message="commit"):
        g = Git(dir)

        if add is not None:
            for f in add:
                g.add(f)

        if all or (add is None and all is None):
            g.add("-a")

        g.commit("-m", message)

    def _commit_yaml(self, y, path, message="save yaml"):
        yaml_save_file(y, path)
        self._commit(os.path.dirname(path), add=[os.path.basename(path)], message=message)

    def _commit_dot_kluctl(self, y, message="update .kluctl.yml"):
        self._commit_yaml(y, os.path.join(self.get_kluctl_project_dir(), ".kluctl.yml"), message=message)

    def copy_deployment(self, source):
        shutil.copytree(source, self.get_deployment_dir())
        self._commit(self.get_deployment_dir(), all=True, message="copy deployment from %s" % source)

    def add_cluster(self, name, context, vars):
        y = {
            "cluster": {
                "name": name,
                "context": context,
                **vars,
            }
        }
        self._commit_yaml(y, os.path.join(self.get_clusters_dir(), "%s.yml" % name), message="add cluster")

    def add_kind_cluster(self, kind_cluster: KindCluster, vars={}):
        if kind_cluster.kubeconfig_path not in self.kubeconfigs:
            self.kubeconfigs.append(kind_cluster.kubeconfig_path)
        context = kind_cluster.kubectl("config", "current-context").strip()
        self.add_cluster(context, context, vars)

    def get_kluctl_project_dir(self):
        return os.path.join(self.base_dir.name, "kluctl-project")

    def get_clusters_dir(self):
        if self.clusters_external:
            return os.path.join(self.base_dir.name, "external-clusters")
        else:
            return os.path.join(self.get_kluctl_project_dir(), "clusters")

    def get_deployment_dir(self):
        if self.deployment_external:
            return os.path.join(self.base_dir.name, "external-deployment")
        else:
            return self.get_kluctl_project_dir()

    def get_sealed_secrets_dir(self):
        if self.sealed_secrets_external:
            return os.path.join(self.base_dir.name, "external-sealed-secrets")
        else:
            return os.path.join(self.get_kluctl_project_dir(), ".sealed-secrets")

    def kluctl(self, *args: str, **kwargs):
        kluctl_path = os.path.join(os.path.dirname(__file__), "..", "..", "cli.py")
        args2 = [kluctl_path]
        args2 += args

        cwd = None
        if self.kluctl_project_external:
            args2 += ["--project-url", "file://%s" % self.get_kluctl_project_dir()]
        else:
            cwd = self.get_kluctl_project_dir()

        if self.local_clusters:
            args2 += ["--local-clusters", self.local_clusters]
        if self.local_deployment:
            args2 += ["--local-deployment", self.local_deployment]
        if self.local_sealed_secrets:
            args2 += ["--local-sealed-secrets", self.local_sealed_secrets]

        if not self.kluctl_project_external:
            cwd = self.get_kluctl_project_dir()

        sep = ";" if sys.platform == "win32" else ":"
        env = os.environ.copy()
        env.update({
            "KUBECONFIG": sep.join(os.path.abspath(str(x)) for x in self.kubeconfigs),
        })

        logger.info("Running kluctl: %s" % " ".join(args2[1:]))

        return subprocess.check_output(
            args2,
            cwd=cwd,
            env=env,
            encoding="utf-8",
            **kwargs,
        )
