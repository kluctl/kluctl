import base64
import contextlib

from pytest_kind import KindCluster

from kluctl.e2e.conftest import recreate_namespace, assert_resource_not_exists, assert_resource_exists
from kluctl.e2e.kluctl_test_project import KluctlTestProject
from kluctl.e2e.kluctl_test_project_helpers import add_job_deployment
from kluctl.utils.dict_utils import get_dict_value
from kluctl.utils.yaml_utils import yaml_load, yaml_dump

hook_script="""
kubectl get configmap -oyaml > /tmp/result.yml
cat /tmp/result.yml
if ! kubectl get secret {name}-result; then
    name="{name}-result"
else
    name="{name}-result2"
fi
kubectl create secret generic $name --from-file=result=/tmp/result.yml
kubectl delete configmap cm2 || true
"""

def add_hook_deployment(p: KluctlTestProject, name, namespace, is_helm, hook, hook_deletion_policy=None):
    a = {}
    if is_helm:
        a["helm.sh/hook"] = hook
        if hook_deletion_policy is not None:
            a["helm.sh/hook-deletion-policy"] = hook_deletion_policy
    else:
        a["kluctl.io/hook"] = hook
    if hook_deletion_policy is not None:
        a["kluctl.io/hook-deletion-policy"] = hook_deletion_policy

    script = hook_script.format(name=name, namespace=namespace)

    add_job_deployment(p, name, name, "bitnami/kubectl:1.21",
                       command=["sh"], args=["-c", script],
                       namespace=namespace, annotations=a)

def add_configmap(p: KluctlTestProject, dir, name, namespace):
    y = {
        "apiVersion": "v1",
        "kind": "ConfigMap",
        "metadata": {
            "name": name,
            "namespace": namespace,
        },
        "data": {},
    }

    p.add_kustomize_resources(dir, {
        "%s.yml" % name: yaml_dump(y)
    })

def get_hook_result(module_kind_cluster: KindCluster, p: KluctlTestProject, secret_name):
    y = module_kind_cluster.kubectl("-n", p.project_name, "get", "secret", secret_name, "-o", "yaml")
    y = yaml_load(y)
    y = get_dict_value(y, "data.result")
    y = base64.b64decode(y).decode("utf-8")
    y = yaml_load(y)
    return y

def get_hook_result_cm_names(module_kind_cluster: KindCluster, p: KluctlTestProject, second=False):
    y = get_hook_result(module_kind_cluster, p, "hook-result" if not second else "hook-result2")
    return [get_dict_value(x, "metadata.name") for x in y["items"]]

@contextlib.contextmanager
def prepare_project(kind_cluster: KindCluster, name, hook, hook_deletion_policy):
    namespace = "hook-%s" % name
    with KluctlTestProject(namespace) as p:
        recreate_namespace(kind_cluster, namespace)

        p.update_kind_cluster(kind_cluster)
        p.update_target("test", "module")

        add_hook_deployment(p, "hook", namespace, is_helm=False, hook=hook, hook_deletion_policy=hook_deletion_policy)
        add_configmap(p, "hook", "cm1", namespace)

        yield p

def ensure_hook_executed(kind_cluster, p: KluctlTestProject):
    try:
        kind_cluster.kubectl("delete", "-n", p.project_name, "secret", "hook-result")
        kind_cluster.kubectl("delete", "-n", p.project_name, "secret", "hook-result2")
    except:
        pass
    p.kluctl("deploy", "--yes", "-t", "test")
    assert_resource_exists(kind_cluster, p.project_name, "ConfigMap/cm1")

def ensure_hook_not_executed(kind_cluster, p: KluctlTestProject):
    try:
        kind_cluster.kubectl("delete", "-n", p.project_name, "secret", "hook-result")
        kind_cluster.kubectl("delete", "-n", p.project_name, "secret", "hook-result2")
    except:
        pass
    p.kluctl("deploy", "--yes", "-t", "test")
    assert_resource_not_exists(kind_cluster, p.project_name, "Secret/hook-result")

def test_hooks_pre_deploy_initial(module_kind_cluster: KindCluster):
    with prepare_project(module_kind_cluster, "pre-deploy-initial", "pre-deploy-initial", None) as p:
        ensure_hook_executed(module_kind_cluster, p)
        assert "cm1" not in get_hook_result_cm_names(module_kind_cluster, p)
        ensure_hook_not_executed(module_kind_cluster, p)

def test_hooks_post_deploy_initial(module_kind_cluster: KindCluster):
    with prepare_project(module_kind_cluster, "post-deploy-initial", "post-deploy-initial", None) as p:
        ensure_hook_executed(module_kind_cluster, p)
        assert "cm1" in get_hook_result_cm_names(module_kind_cluster, p)
        ensure_hook_not_executed(module_kind_cluster, p)

def test_hooks_pre_deploy_upgrade(module_kind_cluster: KindCluster):
    with prepare_project(module_kind_cluster, "pre-deploy-upgrade", "pre-deploy-upgrade", None) as p:
        add_configmap(p, "hook", "cm2", p.project_name)
        ensure_hook_not_executed(module_kind_cluster, p)
        module_kind_cluster.kubectl("delete", "-n", p.project_name, "configmap", "cm1")
        ensure_hook_executed(module_kind_cluster, p)
        assert "cm1" not in get_hook_result_cm_names(module_kind_cluster, p)
        ensure_hook_executed(module_kind_cluster, p)
        assert "cm1" in get_hook_result_cm_names(module_kind_cluster, p)


def test_hooks_post_deploy_upgrade(module_kind_cluster: KindCluster):
    with prepare_project(module_kind_cluster, "post-deploy-upgrade", "post-deploy-upgrade", None) as p:
        add_configmap(p, "hook", "cm2", p.project_name)
        ensure_hook_not_executed(module_kind_cluster, p)
        module_kind_cluster.kubectl("delete", "-n", p.project_name, "configmap", "cm1")
        ensure_hook_executed(module_kind_cluster, p)
        assert "cm1" in get_hook_result_cm_names(module_kind_cluster, p)

def do_test_hooks_pre_deploy(module_kind_cluster: KindCluster, name, hooks):
    with prepare_project(module_kind_cluster, name, hooks, None) as p:
        add_configmap(p, "hook", "cm2", p.project_name)
        ensure_hook_executed(module_kind_cluster, p)
        module_kind_cluster.kubectl("delete", "-n", p.project_name, "configmap", "cm1")
        ensure_hook_executed(module_kind_cluster, p)
        assert "cm1" not in get_hook_result_cm_names(module_kind_cluster, p)
        ensure_hook_executed(module_kind_cluster, p)
        assert "cm1" in get_hook_result_cm_names(module_kind_cluster, p)

def do_test_hooks_post_deploy(module_kind_cluster: KindCluster, name, hooks):
    with prepare_project(module_kind_cluster, name, hooks, None) as p:
        add_configmap(p, "hook", "cm2", p.project_name)
        ensure_hook_executed(module_kind_cluster, p)
        module_kind_cluster.kubectl("delete", "-n", p.project_name, "configmap", "cm1")
        ensure_hook_executed(module_kind_cluster, p)
        assert "cm1" in get_hook_result_cm_names(module_kind_cluster, p)

def do_test_hooks_pre_post_deploy(module_kind_cluster: KindCluster, name, hooks):
    with prepare_project(module_kind_cluster, name, hooks, None) as p:
        add_configmap(p, "hook", "cm2", p.project_name)
        ensure_hook_executed(module_kind_cluster, p)
        assert "cm1" not in get_hook_result_cm_names(module_kind_cluster, p)
        assert "cm2" not in get_hook_result_cm_names(module_kind_cluster, p)
        assert "cm1" in get_hook_result_cm_names(module_kind_cluster, p, True)
        assert "cm2" in get_hook_result_cm_names(module_kind_cluster, p, True)

        ensure_hook_executed(module_kind_cluster, p)
        assert "cm1" in get_hook_result_cm_names(module_kind_cluster, p)
        assert "cm2" not in get_hook_result_cm_names(module_kind_cluster, p)
        assert "cm1" in get_hook_result_cm_names(module_kind_cluster, p, True)
        assert "cm2" in get_hook_result_cm_names(module_kind_cluster, p, True)

def test_hooks_pre_deploy(module_kind_cluster: KindCluster):
    do_test_hooks_pre_deploy(module_kind_cluster, "pre-deploy", "pre-deploy")

def test_hooks_pre_deploy2(module_kind_cluster: KindCluster):
    # same as pre-deploy
    do_test_hooks_pre_deploy(module_kind_cluster, "pre-deploy2", "pre-deploy-initial,pre-deploy-upgrade")

def test_hooks_post_deploy(module_kind_cluster: KindCluster):
    do_test_hooks_post_deploy(module_kind_cluster, "post-deploy", "post-deploy")

def test_hooks_post_deploy2(module_kind_cluster: KindCluster):
    # same as post-deploy
    do_test_hooks_post_deploy(module_kind_cluster, "post-deploy2", "post-deploy-initial,post-deploy-upgrade")

def test_hooks_pre_post_deploy(module_kind_cluster: KindCluster):
    do_test_hooks_pre_post_deploy(module_kind_cluster, "pre-post-deploy", "pre-deploy,post-deploy")

def test_hooks_pre_post_deploy2(module_kind_cluster: KindCluster):
    do_test_hooks_pre_post_deploy(module_kind_cluster, "pre-post-deploy2", "pre-deploy-initial,pre-deploy-upgrade,post-deploy-initial,post-deploy-upgrade")
