# Helper methods for k8s-related tools like kubectl or kustomize
import logging
import os

from kubernetes import config
from kubernetes.client import ApiException

from kluctl.utils.exceptions import CommandError
from kluctl.utils.utils import MyThreadPoolExecutor
from kluctl.utils.yaml_utils import yaml_load_file

logger = logging.getLogger(__name__)

class k8s_cluster_base(object):
    def get_objects(self, group=None, version=None, kind=None, name=None, namespace=None, labels=None, as_table=False):
        return self._get_objects(group, version, kind, name, namespace, labels, as_table)

    def _get_objects(self, group, version, kind, name, namespace, labels, as_table):
        raise NotImplementedError()

    def get_single_object(self, ref):
        l = self.get_objects(version=ref.api_version, kind=ref.kind, name=ref.name, namespace=ref.namespace)

        if not l:
            return None
        if len(l) != 1:
            raise Exception("expected single object, got %d" % len(l))
        return l[0]

    def get_objects_by_object_refs(self, object_refs):
        with MyThreadPoolExecutor(max_workers=32) as executor:
            futures = []
            for ref in object_refs:
                f = executor.submit(self.get_single_object, ref)
                futures.append(f)
            ret = []
            for f in futures:
                r = f.result()
                if r:
                    ret.append(r)
            return ret

    def get_objects_metadata(self, group=None, version=None, kind=None, name=None, namespace=None, labels=None):
        if group is None or kind is None:
            raise ApiException("group/kind must be supplied")
        tables = self.get_objects(group=group, version=version, kind=kind, name=name, namespace=namespace, labels=labels, as_table=True)
        ret = []
        for table, warnings in tables:
            if not table.get("rows", []):
                continue
            for r in table["rows"]:
                o = r["object"]
                o["kind"] = kind
                o["apiVersion"] = group or ""
                if version:
                    if group:
                        o["apiVersion"] += "/"
                    o["apiVersion"] += version
                ret.append((o, warnings))
        return ret


k8s_cluster_instance: k8s_cluster_base = None
cluster = None

def load_cluster_config(cluster_dir, cluster_name, offline=False, dry_run=True):
    if cluster_name is None:
        raise CommandError("Cluster name must be specified!")

    path = os.path.join(cluster_dir, "%s.yml" % cluster_name)
    if not os.path.exists(path):
        raise CommandError("Cluster %s not known" % cluster_name)
    y = yaml_load_file(path)

    global cluster
    global k8s_cluster_instance
    cluster = y['cluster']

    if offline:
        return

    contexts, _ = config.list_kube_config_contexts()

    if not any(x['name'] == cluster['context'] for x in contexts):
        raise CommandError('Context %s for cluster %s not found in kubeconfig' % (cluster['context'], cluster_name))

    if cluster['name'] != cluster_name:
        raise CommandError('Cluster name in %s does not match requested cluster name %s' % (cluster['name'], cluster_name))

    from kluctl.utils.k8s_cluster_real import k8s_cluster_real
    k8s_cluster_instance = k8s_cluster_real(cluster['context'], dry_run)

def get_cluster():
    return cluster

def get_k8s_cluster():
    return k8s_cluster_instance