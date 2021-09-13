from pytest_kind import KindCluster

from kluctl.e2e.conftest import assert_readiness, recreate_namespace
from kluctl.e2e.conftest import assert_resource_not_exists, assert_resource_exists
from kluctl.e2e.kluctl_test_project import KluctlTestProject
from kluctl.e2e.kluctl_test_project_helpers import add_busybox_deployment
from kluctl.utils.dict_utils import get_dict_value

config_map = """
apiVersion: v1
kind: ConfigMap
metadata:
  name: "cm"
  namespace: {namespace}
data:
  cluster_var: {{{{ cluster.cluster_var }}}}
  target_var: {{{{ args.target_var }}}}
"""

def do_test_project(kind_cluster, namespace, **kwargs):
    recreate_namespace(kind_cluster, namespace)

    with KluctlTestProject(**kwargs) as p:
        p.update_kind_cluster(kind_cluster, {"cluster_var": "cluster_value1"})
        p.update_target("test", kind_cluster.name, {"target_var": "target_value1"})
        add_busybox_deployment(p, "busybox", "busybox", namespace=namespace)

        p.kluctl("deploy", "--yes", "-t", "test")
        assert_readiness(kind_cluster, namespace, "Deployment/busybox", 5 * 60)

        assert_resource_not_exists(kind_cluster, namespace, "ConfigMap/cm")
        p.add_kustomize_deployment("cm", resources={"cm.yml": config_map.format(namespace=namespace)})
        p.kluctl("deploy", "--yes", "-t", "test")
        y = assert_resource_exists(kind_cluster, namespace, "ConfigMap/cm")
        assert get_dict_value(y, "data.cluster_var") == "cluster_value1"
        assert get_dict_value(y, "data.target_var") == "target_value1"

        p.update_kind_cluster(kind_cluster, {"cluster_var": "cluster_value2"})
        p.kluctl("deploy", "--yes", "-t", "test")
        y = assert_resource_exists(kind_cluster, namespace, "ConfigMap/cm")
        assert get_dict_value(y, "data.cluster_var") == "cluster_value2"
        assert get_dict_value(y, "data.target_var") == "target_value1"

        p.update_target("test", kind_cluster.name, {"target_var": "target_value2"})
        p.kluctl("deploy", "--yes", "-t", "test")
        y = assert_resource_exists(kind_cluster, namespace, "ConfigMap/cm")
        assert get_dict_value(y, "data.cluster_var") == "cluster_value2"
        assert get_dict_value(y, "data.target_var") == "target_value2"

def test_external_kluctl_project(module_kind_cluster: KindCluster):
    do_test_project(module_kind_cluster, "a", kluctl_project_external=True)

def test_external_clusters_project(module_kind_cluster: KindCluster):
    do_test_project(module_kind_cluster, "b", clusters_external=True)

def test_external_deployment_project(module_kind_cluster: KindCluster):
    do_test_project(module_kind_cluster, "c", deployment_external=True)

def test_external_sealed_secrets_project(module_kind_cluster: KindCluster):
    do_test_project(module_kind_cluster, "d", sealed_secrets_external=True)

def test_all_projects_external(module_kind_cluster: KindCluster):
    do_test_project(module_kind_cluster, "e",
                    kluctl_project_external=True,
                    clusters_external=True,
                    deployment_external=True,
                    sealed_secrets_external=True)
