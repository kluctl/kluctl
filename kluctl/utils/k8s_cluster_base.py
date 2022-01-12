# Helper methods for k8s-related tools like kubectl or kustomize
import logging
import os

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
            return None, []
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
                o, w = f.result()
                if o:
                    ret.append((o, w))
            return ret

    def get_objects_metadata(self, group=None, version=None, kind=None, name=None, namespace=None, labels=None):
        if group is None or kind is None:
            raise ApiException("group/kind must be supplied")
        return self.get_objects(group=group, version=version, kind=kind, name=name, namespace=namespace, labels=labels, as_table=True)

def load_cluster_config(cluster_dir, cluster_name):
    if cluster_name is None:
        raise CommandError("Cluster name must be specified!")

    path = os.path.join(cluster_dir, "%s.yml" % cluster_name)
    if not os.path.exists(path):
        raise CommandError("Cluster %s not known" % cluster_name)
    y = yaml_load_file(path)

    cluster = y['cluster']

    if cluster['name'] != cluster_name:
        raise CommandError('Cluster name in %s does not match requested cluster name %s' % (cluster['name'], cluster_name))

    return cluster

def load_cluster(cluster_config, dry_run=True):
    from kluctl.utils.k8s_cluster_real import k8s_cluster_real
    k8s_cluster = k8s_cluster_real(cluster_config['context'], dry_run)
    return k8s_cluster
