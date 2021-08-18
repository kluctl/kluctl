import logging
import os

from kluctl.utils.exceptions import CommandError
from kluctl.seal.seal_cluster import SealCluster
from kluctl.utils.external_args import parse_args
from kluctl.utils.k8s_cluster_base import load_cluster_config
from kluctl.utils.yaml_utils import yaml_load_file

logger = logging.getLogger(__name__)


def seal_command_for_cluster(cluster_dir, cluster_name, kwargs, load_cluster):
    from kubernetes.dynamic.exceptions import UnauthorizedError
    if load_cluster:
        try:
            load_cluster_config(cluster_dir, cluster_name)
        except UnauthorizedError:
            logger.warning("Failed to authenticate/authorize for kubernetes cluster %s" % cluster_name)
            return
        except Exception as e:
            logger.warning("Failed to load kubernetes cluster %s. Error=%s" % (cluster_name, str(e)))
            return

    for dirpath, dirnames, filenames in os.walk(kwargs["deployment"]):
        if 'sealme-conf.yml' in filenames:
            seal_args = parse_args(kwargs["arg"])
            seal_cluster = SealCluster(kwargs["deployment"], dirpath, kwargs["sealed_secrets_dir"], kwargs["secrets_dir"], seal_args, kwargs["force_reseal"])
            seal_cluster.seal_matrix()

def seal_command(obj, kwargs):
    from kubernetes import config

    cluster_name = obj["cluster_name"]
    cluster_dir = obj["cluster_dir"]
    if cluster_name is not None:
        seal_command_for_cluster(cluster_dir, cluster_name, kwargs, True)
    else:
        cluster_names = []

        contexts, _ = config.list_kube_config_contexts()

        for fname in os.listdir(cluster_dir):
            if fname.endswith('.yml'):
                y = yaml_load_file(os.path.join(cluster_dir, fname))
                for c in contexts:
                    if "cluster" not in y:
                        continue
                    if "context" not in y["cluster"] or "name" not in y["cluster"]:
                        continue
                    if c["name"] == y["cluster"]["context"]:
                        cluster_names.append(y["cluster"]["name"])

        if len(cluster_names) == 0:
            raise CommandError("No known clusters found in kubeconfig")

        for c in cluster_names:
            logger.info('Sealing for cluster \'%s\'' % c)
            seal_command_for_cluster(cluster_dir, c, kwargs, True)
