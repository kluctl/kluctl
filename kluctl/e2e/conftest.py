import contextlib
import logging
import subprocess
import time

import pytest
from pytest_kind import KindCluster

from kluctl.utils.k8s_status_validation import validate_object
from kluctl.utils.yaml_utils import yaml_load

logger = logging.getLogger(__name__)

delete_clusters = True

@contextlib.contextmanager
def kind_cluster_fixture(name):
    cluster = KindCluster(name)
    try:
        if delete_clusters:
            cluster.delete()
    except:
        pass
    cluster.create()
    try:
        yield cluster
    finally:
        if delete_clusters:
            cluster.delete()

@pytest.fixture(scope="function")
def function_kind_cluster(request):
    with kind_cluster_fixture("function") as c:
        yield c

@pytest.fixture(scope="module")
def module_kind_cluster(request):
    with kind_cluster_fixture("module") as c:
        yield c

def recreate_namespace(kind_cluster: KindCluster, namespace):
    try:
        kind_cluster.kubectl("delete", "ns", namespace)
    except:
        pass
    try:
        kind_cluster.kubectl("create", "ns", namespace)
    except:
        pass

def wait_for_readiness(kind_cluster: KindCluster, namespace, resource, timeout):
    logger.info("Waiting for readiness: %s/%s" % (namespace, resource))

    t = time.time()
    while time.time() - t < timeout:
        y = kind_cluster.kubectl("-n", namespace, "get", resource, "-oyaml")
        y = yaml_load(y)

        r = validate_object(y, True)
        if r.ready:
            return True
    return False

def assert_readiness(kind_cluster: KindCluster, namespace, resource, timeout):
    if not wait_for_readiness(kind_cluster, namespace, resource, timeout):
        raise AssertionError("%s/%s did not get ready in time" % (namespace, resource))

def assert_resource_exists(kind_cluster: KindCluster, namespace, resource):
    try:
        y = kind_cluster.kubectl("-n", namespace, "get", resource, "-o", "yaml", stderr=subprocess.PIPE)
        return yaml_load(y)
    except subprocess.CalledProcessError as e:
        assert False, e.stderr

def assert_resource_not_exists(kind_cluster: KindCluster, namespace, resource):
    try:
        kind_cluster.kubectl("-n", namespace, "get", resource, stderr=subprocess.PIPE)
        assert False, "'kubectl get' should not have succeeded"
    except subprocess.CalledProcessError as e:
        assert "(NotFound)" in e.stderr, e.stderr
