from pytest_kind import KindCluster

from kluctl.e2e.conftest import assert_readiness
from kluctl.e2e.kluctl_test_project import KluctlTestProject
from kluctl.e2e.kluctl_test_project_helpers import add_busybox_deployment
from pytest_kind import KindCluster

from kluctl.e2e.conftest import assert_readiness
from kluctl.e2e.kluctl_test_project import KluctlTestProject
from kluctl.e2e.kluctl_test_project_helpers import add_busybox_deployment


def do_test_project(kind_cluster, namespace, **kwargs):
    try:
        kind_cluster.kubectl("create", "ns", namespace)
    except:
        pass

    with KluctlTestProject(**kwargs) as p:
        p.add_kind_cluster(kind_cluster)
        p.add_target("test", kind_cluster.name, {})
        add_busybox_deployment(p, "busybox", "busybox", namespace=namespace)
        p.kluctl("deploy", "--yes", "-t", "test")
        assert_readiness(kind_cluster, namespace, "Deployment/busybox", 5 * 60)

def test_external_kluctl_project(module_kind_cluster: KindCluster):
    do_test_project(module_kind_cluster, "a", kluctl_project_external=True)

def test_external_clusters_project(module_kind_cluster: KindCluster):
    do_test_project(module_kind_cluster, "b", clusters_external=True)

def test_external_deployment_project(module_kind_cluster: KindCluster):
    do_test_project(module_kind_cluster, "c", deployment_external=True)

def test_external_sealed_secrets_project(module_kind_cluster: KindCluster):
    do_test_project(module_kind_cluster, "d", sealed_secrets_external=True)

def test_all_prok(module_kind_cluster: KindCluster):
    do_test_project(module_kind_cluster, "e",
                    kluctl_project_external=True,
                    clusters_external=True,
                    deployment_external=True,
                    sealed_secrets_external=True)
