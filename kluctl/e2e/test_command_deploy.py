from pytest_kind import KindCluster

from kluctl.e2e.conftest import assert_resource_exists, recreate_namespace, assert_resource_not_exists
from kluctl.e2e.kluctl_test_project import KluctlTestProject
from kluctl.e2e.kluctl_test_project_helpers import add_configmap_deployment


def test_command_deploy_simple(kind_cluster: KindCluster):
    with KluctlTestProject("simple") as p:
        recreate_namespace(kind_cluster, "simple")

        p.update_kind_cluster(kind_cluster)
        p.update_target("test", kind_cluster.name)

        add_configmap_deployment(p, "cm", "cm", namespace="simple")
        p.kluctl("deploy", "--yes", "-t", "test")
        assert_resource_exists(kind_cluster, "simple", "ConfigMap/cm")

        add_configmap_deployment(p, "cm2", "cm2", namespace="simple")
        p.kluctl("deploy", "--yes", "-t", "test", "--dry-run")
        assert_resource_not_exists(kind_cluster, "simple", "ConfigMap/cm2")
        p.kluctl("deploy", "--yes", "-t", "test")
        assert_resource_exists(kind_cluster, "simple", "ConfigMap/cm2")
