from pytest_kind import KindCluster

from kluctl.e2e.conftest import recreate_namespace
from kluctl.e2e.kluctl_test_project import KluctlTestProject
from kluctl.e2e.kluctl_test_project_helpers import add_configmap_deployment
from kluctl.utils.dict_utils import get_dict_value
from kluctl.utils.yaml_utils import yaml_load


def prepare_project(module_kind_cluster: KindCluster, p: KluctlTestProject, namespace, with_includes):
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

    if with_includes:
        p.add_deployment_include(".", "include1")
        add_configmap_deployment(p, "include1/icm1", "icm1", namespace=namespace, tags=["itag1", "itag2"])

        p.add_deployment_include(".", "include2")
        add_configmap_deployment(p, "include2/icm2", "icm2", namespace=namespace)
        add_configmap_deployment(p, "include2/icm3", "icm3", namespace=namespace, tags=["itag3", "itag4"])

        p.add_deployment_include(".", "include3", tags=["itag5"])
        add_configmap_deployment(p, "include3/icm4", "icm4", namespace=namespace)
        add_configmap_deployment(p, "include3/icm5", "icm5", namespace=namespace, tags=["itag5", "itag6"])

def assert_exists_helper(kind_cluster, p, namespace, should_exists, add=None, remove=None):
    if add is not None:
        for x in add:
            should_exists.add(x)
    if remove is not None:
        for x in remove:
            if x in should_exists:
                should_exists.remove(x)
    exists = kind_cluster.kubectl("-n", namespace, "get", "configmaps", "-l", "project_name=%s" % p.project_name, "-o", "yaml")
    exists = yaml_load(exists)["items"]
    found = set(get_dict_value(x, "metadata.name") for x in exists)
    assert found == should_exists

def test_inclusion_tags(module_kind_cluster: KindCluster):
    with KluctlTestProject("inclusion") as p:
        prepare_project(module_kind_cluster, p, "inclusion", False)

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

def test_exclusion_tags(module_kind_cluster: KindCluster):
    with KluctlTestProject("exclusion") as p:
        prepare_project(module_kind_cluster, p, "exclusion", False)

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

def test_inclusion_include_dirs(module_kind_cluster: KindCluster):
    with KluctlTestProject("include-dirs") as p:
        prepare_project(module_kind_cluster, p, "include-dirs", True)

        should_exists = set()
        def do_assert_exists(add=None):
            assert_exists_helper(module_kind_cluster, p, "include-dirs", should_exists, add)

        do_assert_exists()

        p.kluctl("deploy", "--yes", "-t", "test", "-I", "itag1")
        do_assert_exists({"icm1"})

        p.kluctl("deploy", "--yes", "-t", "test", "-I", "include2")
        do_assert_exists({"icm2", "icm3"})

        p.kluctl("deploy", "--yes", "-t", "test", "-I", "itag5")
        do_assert_exists({"icm4", "icm5"})

def test_inclusion_kustomize_dirs(module_kind_cluster: KindCluster):
    with KluctlTestProject("include-dirs") as p:
        prepare_project(module_kind_cluster, p, "include-dirs", True)

        should_exists = set()
        def do_assert_exists(add=None):
            assert_exists_helper(module_kind_cluster, p, "include-dirs", should_exists, add)

        do_assert_exists()

        p.kluctl("deploy", "--yes", "-t", "test", "--include-kustomize-dir", "include1/icm1")
        do_assert_exists({"icm1"})

        p.kluctl("deploy", "--yes", "-t", "test", "--include-kustomize-dir", "include2/icm3")
        do_assert_exists({"icm3"})

        p.kluctl("deploy", "--yes", "-t", "test", "--exclude-kustomize-dir", "include3/icm5")
        do_assert_exists(set(p.list_kustomize_deployment_pathes()) - {"icm5"})

def test_inclusion_prune(module_kind_cluster: KindCluster):
    with KluctlTestProject("inclusion-prune") as p:
        prepare_project(module_kind_cluster, p, "inclusion-prune", False)

        should_exists = set()
        def do_assert_exists(add=None, remove=None):
            assert_exists_helper(module_kind_cluster, p, "inclusion-prune", should_exists, add, remove)

        p.kluctl("deploy", "--yes", "-t", "test")
        do_assert_exists(p.list_kustomize_deployment_pathes())

        p.delete_kustomize_deployment("cm1")
        p.kluctl("prune", "--yes", "-t", "test", "-I", "non-existent-tag")
        do_assert_exists()

        p.kluctl("prune", "--yes", "-t", "test", "-I", "cm1")
        do_assert_exists(remove={"cm1"})

        p.delete_kustomize_deployment("cm2")
        p.kluctl("prune", "--yes", "-t", "test", "-E", "cm2")
        do_assert_exists()

        p.delete_kustomize_deployment("cm3")
        p.kluctl("prune", "--yes", "-t", "test", "--exclude-kustomize-dir", "cm3")
        do_assert_exists(remove={"cm2"})

        p.kluctl("prune", "--yes", "-t", "test")
        do_assert_exists(remove={"cm3"})

def test_inclusion_delete(module_kind_cluster: KindCluster):
    with KluctlTestProject("inclusion-delete") as p:
        prepare_project(module_kind_cluster, p, "inclusion-delete", False)

        should_exists = set()
        def do_assert_exists(add=None, remove=None):
            assert_exists_helper(module_kind_cluster, p, "inclusion-delete", should_exists, add, remove)

        p.kluctl("deploy", "--yes", "-t", "test")
        do_assert_exists(p.list_kustomize_deployment_pathes())

        p.kluctl("delete", "--yes", "-t", "test", "-I", "non-existent-tag")
        do_assert_exists()

        p.kluctl("delete", "--yes", "-t", "test", "-I", "cm1")
        do_assert_exists(remove={"cm1"})

        p.kluctl("delete", "--yes", "-t", "test", "-E", "cm2")
        do_assert_exists(remove=set(p.list_kustomize_deployment_pathes()) - {"cm1", "cm2"})
