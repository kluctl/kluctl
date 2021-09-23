import os
import shutil
from tempfile import TemporaryDirectory

from pytest_kind import KindCluster

from kluctl import get_kluctl_package_dir
from kluctl.e2e.conftest import assert_readiness, assert_resource_exists, assert_resource_not_exists
from kluctl.e2e.kluctl_test_project import KluctlTestProject
from kluctl.utils.yaml_utils import yaml_save_file, yaml_load_file

dummy_configmap = """
apiVersion: v1
kind: ConfigMap
metadata:
  name: dummy-configmap
  namespace: kube-system
data:
  dummy: value
"""

def test_command_bootstrap(kind_cluster: KindCluster):
    bootstrap_path = os.path.join(get_kluctl_package_dir(), "bootstrap")

    with KluctlTestProject("bootstrap") as p:
        p.update_kind_cluster(kind_cluster)
        p.update_target("test", kind_cluster.name)
        p.kluctl("bootstrap", "--yes", "--cluster", "pytest-kind")
        assert_readiness(kind_cluster, "kube-system", "Deployment/sealed-secrets", 60 * 5)

    # test upgrades
    with TemporaryDirectory() as tmpdir:
        shutil.copytree(bootstrap_path, tmpdir, dirs_exist_ok=True)

        with open(os.path.join(tmpdir, "sealed-secrets/dummy.yml"), mode="wt") as f:
            f.write(dummy_configmap)
        k = yaml_load_file(os.path.join(tmpdir, "sealed-secrets/kustomization.yml"))
        k["resources"].append("dummy.yml")
        yaml_save_file(k, os.path.join(tmpdir, "sealed-secrets/kustomization.yml"))

        with KluctlTestProject("bootstrap", local_deployment=tmpdir) as p:
            p.update_kind_cluster(kind_cluster)
            p.update_target("test", kind_cluster.name)
            p.kluctl("bootstrap", "--yes", "--cluster", "pytest-kind")
            assert_resource_exists(kind_cluster, "kube-system", "ConfigMap/dummy-configmap")

    # test pruning
    with KluctlTestProject("bootstrap") as p:
        p.update_kind_cluster(kind_cluster)
        p.update_target("test", kind_cluster.name)
        p.kluctl("bootstrap", "--yes", "--cluster", "pytest-kind")
        assert_resource_not_exists(kind_cluster, "kube-system", "ConfigMap/dummy-configmap")
