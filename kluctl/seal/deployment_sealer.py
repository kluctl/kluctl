import logging
import os

from kluctl.seal.sealer import Sealer
from kluctl.utils.dict_utils import get_dict_value
from kluctl.utils.exceptions import InvalidKluctlProjectConfig

logger = logging.getLogger(__name__)

SEALME_EXT = ".sealme"

class SealFailedException(Exception):
    pass

class DeploymentSealer:
    def __init__(self, deployment_project, deployment_collection, cluster_vars, sealed_secrets_dir, force_reseal):
        from kluctl.deployment.deployment_project import DeploymentProject
        from kluctl.deployment.deployment_collection import DeploymentCollection
        self.deployment_project: DeploymentProject = deployment_project
        self.deployment_collection: DeploymentCollection = deployment_collection
        self.sealed_secrets_dir = os.path.abspath(sealed_secrets_dir)
        self.guess_sealed_secrets_controller()
        self.sealer = Sealer(cluster_vars, self.sealed_secrets_namespace, self.sealed_secrets_controller_name, force_reseal)

    def guess_sealed_secrets_controller(self):
        self.sealed_secrets_namespace = "kube-system"
        self.sealed_secrets_controller_name = "sealed-secrets"

    def seal_deployment(self):
        output_suffix = get_dict_value(self.deployment_project.conf, "sealedSecrets.outputPattern")
        if output_suffix is None:
            raise InvalidKluctlProjectConfig("sealedSecrets.outputPattern is not defined")

        for dirpath, dirnames, filenames in os.walk(self.deployment_collection.tmpdir):
            rel_dir = os.path.relpath(dirpath, self.deployment_collection.tmpdir)
            target_dir = os.path.join(self.sealed_secrets_dir, rel_dir)
            for f in filenames:
                if f.endswith(SEALME_EXT):
                    source_file = os.path.join(dirpath, f)
                    target_file = os.path.join(target_dir, output_suffix, f)
                    target_file = target_file[:-len(SEALME_EXT)]
                    self.seal_file(source_file, target_file)

    def seal_file(self, source_file, target_file):
        try:
            self.sealer.seal_file(source_file, target_file)
        except Exception as e:
            err_txt = ''
            if not isinstance(e, SealFailedException):
                err_txt = ' ' + str(e)
            logger.error(f"Failed sealing {os.path.basename(source_file)}.{err_txt}")
