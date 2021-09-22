import logging
import os
import shutil
import subprocess
import sys
from pathlib import Path
from tempfile import TemporaryDirectory

from git import Git
from pytest_kind import KindCluster

from kluctl.utils.dict_utils import copy_dict, set_dict_value, del_dict_value
from kluctl.utils.run_helper import run_helper
from kluctl.utils.yaml_utils import yaml_save_file, yaml_load_file

logger = logging.getLogger(__name__)


class KluctlTestProject:
    def __init__(self, project_name, kluctl_project_external=False,
                 clusters_external=False, deployment_external=False, sealed_secrets_external=False,
                 local_clusters=None, local_deployment=None, local_sealed_secrets=None):
        self.project_name = project_name
        self.kluctl_project_external = kluctl_project_external
        self.clusters_external = clusters_external
        self.deployment_external = deployment_external
        self.sealed_secrets_external = sealed_secrets_external
        self.local_clusters = local_clusters
        self.local_deployment = local_deployment
        self.local_sealed_secrets = local_sealed_secrets
        self.kubeconfigs = []

    def __enter__(self):
        self.base_dir = TemporaryDirectory(prefix="kluctl-e2e-")

        os.makedirs(self.get_kluctl_project_dir(), exist_ok=True)
        os.makedirs(os.path.join(self.get_clusters_dir(), "clusters"), exist_ok=True)
        os.makedirs(os.path.join(self.get_sealed_secrets_dir(), ".sealed-secrets"), exist_ok=True)
        os.makedirs(self.get_deployment_dir(), exist_ok=True)

        self._git_init(self.get_kluctl_project_dir())
        if self.clusters_external:
            self._git_init(self.get_clusters_dir())
        if self.deployment_external:
            self._git_init(self.get_deployment_dir())
        if self.sealed_secrets_external:
            self._git_init(self.get_sealed_secrets_dir())

        def do_update_kluctl(y):
            if self.clusters_external:
                y["clusters"] = {
                    "project": "file://%s" % self.get_clusters_dir(),
                }
            if self.deployment_external:
                y["deployment"] = {
                    "project": "file://%s" % self.get_deployment_dir(),
                }
            if self.sealed_secrets_external:
                y["sealedSecrets"] = {
                    "project": "file://%s" % self.get_sealed_secrets_dir(),
                }
            return y

        self.update_kluctl_yaml(do_update_kluctl)
        self.update_deployment_yaml(".", lambda y: y)

        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        return self.base_dir.__exit__(exc_type, exc_val, exc_tb)

    def _git_init(self, dir):
        os.makedirs(dir, exist_ok=True)
        g = Git(dir)
        g.init()
        with open(os.path.join(dir, ".dummy"), "wt") as f:
            f.write("dummy")
        g.add(".dummy")
        g.commit("-m", "initial")

    def _commit(self, dir, add=None, all=None, message="commit"):
        assert self.base_dir.name in dir

        g = Git(dir)

        if add is not None:
            for f in add:
                g.add(f)

        if all or (add is None and all is None):
            g.add(".")

        g.commit("-m", message)

    def _commit_yaml(self, y, path, message="save yaml"):
        yaml_save_file(y, path)
        self._commit(os.path.dirname(path), add=[os.path.basename(path)], message=message)

    def update_yaml(self, path, update, message=None):
        assert self.base_dir.name in path
        try:
            y = yaml_load_file(path)
            orig_y = copy_dict(y)
        except:
            y = {}
            orig_y = None
        y = update(y)
        if y == orig_y:
            return

        yaml_save_file(y, path)

        if message is None:
            message = "update %s" % os.path.relpath(path, self.base_dir.name)
        self._commit_yaml(y, path, message=message)

    def update_kluctl_yaml(self, update):
        self.update_yaml(os.path.join(self.get_kluctl_project_dir(), ".kluctl.yml"), update)

    def update_deployment_yaml(self, dir, update):
        def do_update(y):
            if dir == ".":
                set_dict_value(y, "commonLabels.project_name", self.project_name)
                set_dict_value(y, "deleteByLabels.project_name", self.project_name)
            y = update(y)
            return y
        self.update_yaml(os.path.join(self.get_deployment_dir(), dir, "deployment.yml"), do_update)

    def get_deployment_yaml(self, dir):
        path = os.path.join(self.get_deployment_dir(), dir, "deployment.yml")
        return yaml_load_file(path)

    def list_kustomize_deployments(self, dir="."):
        ret = []
        y = self.get_deployment_yaml(dir)
        for inc in y.get("includes", []):
            ret += self.list_kustomize_deployments(os.path.join(dir, inc["path"]))

        ret += self.get_deployment_yaml(dir).get("kustomizeDirs", [])
        return ret

    def list_kustomize_deployment_pathes(self, dir="."):
        return [x["path"] for x in self.list_kustomize_deployments(dir)]


    def update_kustomize_deployment(self, dir, update):
        path = os.path.join(self.get_deployment_dir(), dir, "kustomization.yml")
        self.update_yaml(path, update, message="Update kustomization.yml for %s" % dir)

    def copy_deployment(self, source):
        shutil.copytree(source, self.get_deployment_dir())
        self._commit(self.get_deployment_dir(), all=True, message="copy deployment from %s" % source)

    def copy_kustomize_dir(self, source, target, add_to_deployment):
        p = os.path.join(self.get_deployment_dir(), target)
        shutil.copytree(source, p)
        self._commit(p, all=True, message="copy kustomize dir from %s to %s" % (source, target))

    def update_cluster(self, name, context, vars):
        path = os.path.join(self.get_clusters_dir(), "clusters", "%s.yml" % name)
        def do_update(y):
            y = {
                "cluster": {
                    "name": name,
                    "context": context,
                    **vars,
                }
            }
            return y
        self.update_yaml(path, do_update, message="add/update cluster %s" % name)

    def update_kind_cluster(self, kind_cluster: KindCluster, vars={}):
        if kind_cluster.kubeconfig_path not in self.kubeconfigs:
            self.kubeconfigs.append(kind_cluster.kubeconfig_path)
        context = kind_cluster.kubectl("config", "current-context").strip()
        self.update_cluster(kind_cluster.name, context, vars)

    def update_target(self, name, cluster, args={}):
        def do_update(y):
            targets = y.get("targets", [])
            targets = [x for x in targets if x["name"] != name]
            y["targets"] = targets
            targets.append({
                "name": name,
                "cluster": cluster,
                "args": args,
            })
            return y
        self.update_kluctl_yaml(do_update)

    def add_deployment_include(self, dir, include, tags=None):
        def do_update(y):
            includes = y.setdefault("includes", [])
            if any(x["path"] == include for x in includes):
                return y
            o = {
                "path": include,
            }
            if tags is not None:
                o["tags"] = tags
            includes.append(o)
            return y
        self.update_deployment_yaml(dir, do_update)

    def add_deployment_includes(self, dir):
        p = ["."]
        for x in Path(dir).parts:
            self.add_deployment_include(os.path.join(*p), x)
            p.append(x)

    def add_kustomize_deployment(self, dir, resources, tags=None):
        deployment_dir = os.path.dirname(dir)
        if deployment_dir != "":
            self.add_deployment_includes(deployment_dir)

        abs_deployment_dir = os.path.join(self.get_deployment_dir(), deployment_dir)
        abs_kustomize_dir = os.path.join(self.get_deployment_dir(), dir)

        os.makedirs(abs_kustomize_dir, exist_ok=True)

        y = {
            "apiVersion": "kustomize.config.k8s.io/v1beta1",
            "kind": "Kustomization",
        }
        yaml_save_file(y, os.path.join(abs_kustomize_dir, "kustomization.yml"))
        self._commit(abs_deployment_dir, all=True, message="add kustomize deployment %s" % dir)

        self.add_kustomize_resources(dir, resources)

        def do_update(y):
            kustomize_dirs = y.setdefault("kustomizeDirs", [])
            o = {
                "path": os.path.basename(dir)
            }
            if tags is not None:
                o["tags"] = tags
            kustomize_dirs.append(o)
            return y
        self.update_deployment_yaml(deployment_dir, do_update)

    def add_kustomize_resources(self, dir, resources):
        def do_update(y):
            y.setdefault("resources", [])
            y["resources"] += list(resources.keys())
            for name, content in resources.items():
                with open(os.path.join(self.get_deployment_dir(), dir, name), "wt") as f:
                    f.write(content)
            return y
        self.update_kustomize_deployment(dir, do_update)

    def delete_kustomize_deployment(self, dir):
        deployment_dir = os.path.dirname(dir)

        def do_update(y):
            for i in range(len(y.get("kustomizeDirs", []))):
                if y["kustomizeDirs"][i]["path"] == os.path.basename(dir):
                    del y["kustomizeDirs"][i]
                    break
            return y
        self.update_deployment_yaml(deployment_dir, do_update)

    def get_kluctl_project_dir(self):
        return os.path.join(self.base_dir.name, "kluctl-project")

    def get_clusters_dir(self):
        if self.clusters_external:
            return os.path.join(self.base_dir.name, "external-clusters")
        else:
            return self.get_kluctl_project_dir()

    def get_deployment_dir(self):
        if self.deployment_external:
            return os.path.join(self.base_dir.name, "external-deployment")
        else:
            return self.get_kluctl_project_dir()

    def get_sealed_secrets_dir(self):
        if self.sealed_secrets_external:
            return os.path.join(self.base_dir.name, "external-sealed-secrets")
        else:
            return self.get_kluctl_project_dir()

    def kluctl(self, *args: str, check_rc=True, **kwargs):
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

        sep = ";" if sys.platform == "win32" else ":"
        env = os.environ.copy()
        env.update({
            "KUBECONFIG": sep.join(os.path.abspath(str(x)) for x in self.kubeconfigs),
        })

        logger.info("Running kluctl: %s" % " ".join(args2[1:]))

        def do_log(lines):
            for l in lines:
                logger.info(l)

        rc, stdout, stderr = run_helper(args2, cwd=cwd, env=env, stdout_func=do_log, stderr_func=do_log, line_mode=True, return_std=True)
        assert not check_rc or rc == 0, "rc=%d" % rc
        return rc, stdout, stderr
