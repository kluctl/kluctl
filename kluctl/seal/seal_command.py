import logging
import sys

from jinja2 import TemplateError

from kluctl.cli.utils import project_target_command_context
from kluctl.kluctl_project.kluctl_project import load_kluctl_project_from_args
from kluctl.kluctl_project.secrets import SecretsLoader
from kluctl.seal.deployment_sealer import DeploymentSealer
from kluctl.utils.exceptions import InvalidKluctlProjectConfig, CommandError
from kluctl.utils.jinja2_utils import render_dict_strs, print_template_error
from kluctl.utils.dict_utils import merge_dict, get_dict_value

logger = logging.getLogger(__name__)

def find_secrets_entry(kluctl_project, name):
    for e2 in get_dict_value(kluctl_project.config, "secretsConfig.secretSets", []):
        if name == e2["name"]:
            return e2
    raise InvalidKluctlProjectConfig("Secret Set with name %s was not found" % name)

def merge_deployment_vars(deployment, jinja_var):
    for d in deployment.get_children(True, True):
        merge_dict(d.jinja_vars, jinja_var, False)

def load_secrets(kluctl_project, target, secrets_loader, deployment):
    secrets = {}
    for e in get_dict_value(target, "sealingConfig.secretSets", []):
        secrets_entry = find_secrets_entry(kluctl_project, e)
        for source in secrets_entry.get("sources", []):
            rendered_source = render_dict_strs(source, deployment.jinja_vars)
            s = secrets_loader.load_secrets(rendered_source)
            merge_dict(secrets, s, False)

    merge_deployment_vars(deployment, {"secrets": secrets})

def seal_command_for_target(kwargs, kluctl_project, target, sealing_config, secrets_loader):
    logger.info("Sealing for target %s" % target["name"])

    # pass for_seal=True so that .sealme files are rendered as well
    with project_target_command_context(kwargs, kluctl_project, target, for_seal=True) as cmd_ctx:
        load_secrets(kluctl_project, target, secrets_loader, cmd_ctx.deployment)

        s = DeploymentSealer(cmd_ctx.deployment, cmd_ctx.deployment_collection, cmd_ctx.cluster_vars,
                             cmd_ctx.kluctl_project.sealed_secrets_dir, kwargs["force_reseal"])
        s.seal_deployment()

def seal_command(obj, kwargs):
    with load_kluctl_project_from_args(kwargs) as kluctl_project:
        secrets_loader = SecretsLoader(kluctl_project, kwargs["secrets_dir"])

        base_targets = []
        for target in kluctl_project.targets:
            if kwargs["target"] is not None and kwargs["target"] != target["name"]:
                continue
            sealing_config = target.get("sealingConfig")
            if sealing_config is None:
                logger.warning("Target %s has no sealingConfig" % target["name"])
                continue

            dynamic_sealing = sealing_config.get("dynamicSealing", True)
            if not dynamic_sealing and "baseTarget" in target:
                target = target["baseTarget"]
                if any(target is x for x in base_targets):
                    # Skip this target as it was already sealed
                    continue
                base_targets.append(target)

            try:
                seal_command_for_target(kwargs, kluctl_project, target, sealing_config, secrets_loader)
            except TemplateError as e:
                print_template_error(e)
            except CommandError as e:
                print(e, file=sys.stderr)
            except Exception as e:
                logger.exception("Sealing for target %s failed. Error=%s" % (target["name"], str(e)))
