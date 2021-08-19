import logging

from kluctl.cli.utils import project_target_command_context, CommandContext
from kluctl.kluctl_project.kluctl_project import load_kluctl_project_from_args
from kluctl.kluctl_project.secrets import SecretsLoader
from kluctl.seal.deployment_sealer import DeploymentSealer
from kluctl.utils.exceptions import InvalidKluctlProjectConfig
from kluctl.utils.utils import merge_dict

logger = logging.getLogger(__name__)

def find_secrets_entry(kluctl_project, name):
    for e2 in kluctl_project.config.get("secrets", {}).get("secretSets", []):
        if name == e2["name"]:
            return e2
    raise InvalidKluctlProjectConfig("Secret Set with name %s was not found" % e)


def seal_command_for_target(kwargs, kluctl_project, target, secrets_loader):
    secrets = {}
    for e in target.get("secretSets", []):
        secrets_entry = find_secrets_entry(kluctl_project, e)
        s = secrets_loader.load_secrets(secrets_entry)
        merge_dict(secrets, s, False)

    # pass for_seal=True so that .sealme files are rendered as well
    with project_target_command_context(kwargs, kluctl_project, target, for_seal=True, secrets=secrets) as cmd_ctx:
        seal_command_for_target(kwargs, cmd_ctx)

        s = DeploymentSealer(cmd_ctx.deployment, cmd_ctx.deployment_collection, cmd_ctx.cluster_vars,
                             cmd_ctx.kluctl_project.sealed_secrets_dir, kwargs["force_reseal"])
        s.seal_deployment()

def seal_command(obj, kwargs):
    with load_kluctl_project_from_args(kwargs) as kluctl_project:
        kluctl_project.load(True)
        kluctl_project.load_targets()

        secrets_loader = SecretsLoader(kluctl_project, kwargs["secrets_dir"])

        for target in kluctl_project.targets:
            if kwargs["target"] is not None and kwargs["target"] != target["name"]:
                continue

            try:
                seal_command_for_target(kwargs, kluctl_project, target, secrets_loader)
            except Exception as e:
                logger.warning("Sealing for target %s failed. Error=%s" % (target["name"], str(e)))
