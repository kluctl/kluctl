import contextlib
import logging
import os
import subprocess
from tempfile import TemporaryDirectory
from typing import ContextManager

from pytest_kind import KindCluster

from kluctl.utils.yaml_utils import yaml_save_file

logger = logging.getLogger(__name__)


class KluctlTestProject:
    def __init__(self, deployment_path, clusters_path, sealed_secrets_path, kubeconfig_path):
        self.deployment_path = deployment_path
        self.clusters_path = clusters_path
        self.sealed_secrets_path = sealed_secrets_path
        self.kubeconfig_path = kubeconfig_path
        self.cluster_name = None

    def add_cluster(self, name, context, vars):
        y = {
            "cluster": {
                "name": name,
                "context": context,
                **(vars if vars is not None else {})
            },
        }
        path = os.path.join(self.clusters_path, "%s.yml" % name)
        yaml_save_file(y, path)
        self.cluster_name = name

    def kluctl(self, *args: str, **kwargs):
        kluctl_path = os.path.join(os.path.dirname(__file__), "..", "..", "cli.py")
        args2 = [kluctl_path]
        args2 += args

        if self.deployment_path is not None:
            args2 += ["--local-deployment", self.deployment_path]
        if self.clusters_path is not None:
            args2 += ["--local-clusters", self.clusters_path]
        if self.sealed_secrets_path is not None:
            args2 += ["--local-sealed-secrets", self.sealed_secrets_path]

        env = os.environ.copy()
        env.update({
            "KUBECONFIG": str(self.kubeconfig_path),
        })

        logger.info("Running kluctl: %s" % " ".join(args2[1:]))

        return subprocess.check_output(
            args2,
            env=env,
            encoding="utf-8",
            **kwargs,
        )


@contextlib.contextmanager
def kluctl_project_context(kind_cluster: KindCluster, deployment_path, cluster_name=None, cluster_vars=None) -> ContextManager[KluctlTestProject]:
    with TemporaryDirectory() as tmp:
        clusters_path = os.path.join(tmp, "clusters")
        sealed_secrets_path = os.path.join(tmp, ".sealed-secrets")

        os.makedirs(clusters_path, exist_ok=True)
        os.makedirs(sealed_secrets_path, exist_ok=True)

        context = kind_cluster.kubectl("config", "current-context").strip()
        if cluster_name is None:
            cluster_name = context

        p = KluctlTestProject(deployment_path,
                              clusters_path,
                              sealed_secrets_path,
                              kind_cluster.kubeconfig_path)
        p.add_cluster(cluster_name, context, cluster_vars)
        try:
            yield p
        finally:
            pass
