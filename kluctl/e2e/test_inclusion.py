from pytest_kind import KindCluster

from kluctl.e2e.conftest import assert_resource_exists, recreate_namespace, assert_resource_not_exists
from kluctl.e2e.kluctl_test_project import KluctlTestProject
from kluctl.e2e.kluctl_test_project_helpers import add_configmap_deployment

def prepare_project(module_kind_cluster: KindCluster, p: KluctlTestProject, namespace):
    recreate_namespace(module_kind_cluster, namespace)

    p.update_kind_cluster(module_kind_cluster)
    p.update_target("test", "module")
    add_configmap_deployment(p, "cm1", "cm1", namespace=namespace)
    add_configmap_deployment(p, "cm2", "cm2", namespace=namespace)
    add_configmap_deployment(p, "cm3", "cm3", namespace=namespace, tags=["tag1", "tag2"])
    add_configmap_deployment(p, "cm4", "cm4", namespace=namespace, tags=["tag1", "tag3"])
    add_configmap_deployment(p, "cm5", "cm5", namespace=namespace, tags=["tag1", "tag4"])
    add_configmap_deployment(p, "cm6", "cm6", namespace=namespace, tags=["tag1", "tag5"])
    add_configmap_deployment(p, "cm7", "cm7", namespace=namespace, tags=["tag1", "tag6"])

def assert_exists_helper(kind_cluster, p, namespace, should_exists, add=None):
    if add is not None:
        for x in add:
            should_exists.add(x)
    for x in p.get_deployment_yaml(".")["kustomizeDirs"]:
        if x["path"] in should_exists:
            assert_resource_exists(kind_cluster, namespace, "ConfigMap/%s" % x["path"])
        else:
            assert_resource_not_exists(kind_cluster, namespace, "ConfigMap/%s" % x["path"])

def test_inclusion_exclusion(module_kind_cluster: KindCluster):
    with KluctlTestProject("inclusion") as p:
        prepare_project(module_kind_cluster, p, "inclusion")

        should_exists = set()
        def do_assert_exists(add=None):
            assert_exists_helper(module_kind_cluster, p, "inclusion", should_exists, add)

        do_assert_exists()

        # test default tags
        p.kluctl("deploy", "--yes", "-t", "test", "-I", "cm1")
        do_assert_exists({"cm1"})
        p.kluctl("deploy", "--yes", "-t", "test", "-I", "cm2")
        do_assert_exists({"cm2"})

        # cm3/cm4 don't have default tags, so they should not deploy
        p.kluctl("deploy", "--yes", "-t", "test", "-I", "cm3")
        do_assert_exists()

        # but with tag2, at least cm3 should deploy
        p.kluctl("deploy", "--yes", "-t", "test", "-I", "tag2")
        do_assert_exists({"cm3"})

        # let's try 2 tags at once
        p.kluctl("deploy", "--yes", "-t", "test", "-I", "tag3", "-I", "tag4")
        do_assert_exists({"cm4", "cm5"})

        # And now let's try a tag that matches all non-default ones
        p.kluctl("deploy", "--yes", "-t", "test", "-I", "tag1")
        do_assert_exists({"cm6", "cm7"})

def test_exclusion_exclusion(module_kind_cluster: KindCluster):
    with KluctlTestProject("exclusion") as p:
        prepare_project(module_kind_cluster, p, "exclusion")

        should_exists = set()
        def do_assert_exists(add=None):
            assert_exists_helper(module_kind_cluster, p, "exclusion", should_exists, add)

        do_assert_exists()

        # Exclude everything except cm1
        p.kluctl("deploy", "--yes", "-t", "test", "-E", "cm2", "-E", "tag1")
        do_assert_exists({"cm1"})

        # Test that exclusion has precedence over inclusion
        p.kluctl("deploy", "--yes", "-t", "test", "-E", "cm2", "-E", "tag1", "-I", "cm2")
        do_assert_exists()

        # Test that exclusion has precedence over inclusion
        p.kluctl("deploy", "--yes", "-t", "test", "-I", "tag1", "-E", "tag6")
        do_assert_exists({"cm3", "cm4", "cm5", "cm6"})
