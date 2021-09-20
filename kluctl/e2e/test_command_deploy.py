from pytest_kind import KindCluster

from kluctl.e2e.conftest import assert_resource_exists, recreate_namespace
from kluctl.e2e.kluctl_test_project import KluctlTestProject
from kluctl.e2e.kluctl_test_project_helpers import add_configmap_deployment


def test_command_deploy_simple(module_kind_cluster: KindCluster):
    with KluctlTestProject("simple") as p:
        recreate_namespace(module_kind_cluster, "simple")

        p.update_kind_cluster(module_kind_cluster)
        p.update_target("test", "module")

        add_configmap_deployment(p, "cm", "cm", namespace="simple")
        p.kluctl("deploy", "--yes", "-t", "test")

        assert_resource_exists(module_kind_cluster, "simple", "ConfigMap/cm")
