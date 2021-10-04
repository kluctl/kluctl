import contextlib
import dataclasses
import tempfile
from typing import ContextManager

from kluctl.kluctl_project.kluctl_project import load_kluctl_project_from_args, KluctlProject
from kluctl.utils.dict_utils import merge_dict, get_dict_value
from kluctl.utils.exceptions import CommandError
from kluctl.deployment.deployment_collection import DeploymentCollection
from kluctl.deployment.deployment_project import DeploymentProject
from kluctl.image_registries import init_image_registries
from kluctl.deployment.images import Images
from kluctl.utils.external_args import parse_args
from kluctl.utils.inclusion import Inclusion
from kluctl.utils.k8s_cluster_base import load_cluster_config, k8s_cluster_base
from kluctl.utils.utils import get_tmp_base_dir
from kluctl.utils.yaml_utils import yaml_load_file


def build_jinja_vars(cluster_vars):
    jinja_vars = {
        'cluster': cluster_vars,
    }

    return jinja_vars

def build_deploy_images(force_offline, kwargs):
    image_registries = None
    if not kwargs.get("no_registries", False):
        image_registries = init_image_registries()
    images = Images(image_registries)
    offline = force_offline or kwargs.get("offline", False)
    images.update_images = kwargs.get("update_images", False) and not offline
    images.no_registries = kwargs.get("no_registries", False) or offline
    return images

def build_fixed_image_entry_from_arg(arg):
    s = arg.split('=')
    if len(s) != 2:
        raise CommandError("--fixed-image expects 'image<:namespace:deployment:container>=result'")
    image = s[0]
    result = s[1]

    s = image.split(":")
    e = {
        "image": s[0],
        "resultImage": result,
    }
    if len(s) >= 2:
        e["namespace"] = s[1]
    if len(s) >= 3:
        e["deployment"] = s[2]
    if len(s) >= 4:
        e["container"] = s[3]
    if len(s) >= 5:
        raise CommandError("--fixed-image expects 'image<:namespace:deployment:container>=result'")
    return e

def load_fixed_images(kwargs):
    ret = []
    if kwargs.get("fixed_images_file"):
        y = yaml_load_file(kwargs["fixed_images_file"])
        ret += y.get("images", [])

    for fi in kwargs.get("fixed_image", []):
        e = build_fixed_image_entry_from_arg(fi)
        ret.append(e)
    return ret


def parse_inclusion(kwargs):
    inclusion = Inclusion()
    for tag in kwargs.get("include_tag", []):
        inclusion.add_include("tag", tag)
    for tag in kwargs.get("exclude_tag", []):
        inclusion.add_exclude("tag", tag)
    for dir in kwargs.get("include_kustomize_dir", []):
        inclusion.add_include("kustomize_dir", dir)
    for dir in kwargs.get("exclude_kustomize_dir", []):
        inclusion.add_exclude("kustomize_dir", dir)
    return inclusion

@dataclasses.dataclass
class CommandContext:
    kluctl_project: KluctlProject
    target: dict
    cluster_vars: dict
    k8s_cluster: k8s_cluster_base
    deployment: DeploymentProject
    deployment_collection: DeploymentCollection
    images: Images

@contextlib.contextmanager
def project_command_context(kwargs,
                            force_offline_images=False,
                            force_offline_kubernetes=False) -> ContextManager[CommandContext]:
    with load_kluctl_project_from_args(kwargs) as kluctl_project:
        target = None
        if kwargs["target"]:
            target = kluctl_project.find_target(kwargs["target"])

        with project_target_command_context(kwargs, kluctl_project, target,
                                            force_offline_images=force_offline_images,
                                            force_offline_kubernetes=force_offline_kubernetes) as cmd_ctx:
            yield cmd_ctx

@contextlib.contextmanager
def project_target_command_context(kwargs, kluctl_project, target,
                                   force_offline_images=False,
                                   force_offline_kubernetes=False,
                                   for_seal=False) -> ContextManager[CommandContext]:

    cluster_name = kwargs["cluster"]
    if not cluster_name:
        if not target:
            raise CommandError("You must specify an existing --cluster when not providing a --target")
        cluster_name = target["cluster"]

    cluster_vars, k8s_cluster = load_cluster_config(kluctl_project.clusters_dir, cluster_name,
                                                    dry_run=kwargs.get("dry_run", True),
                                                    offline=force_offline_kubernetes)

    jinja_vars = build_jinja_vars(cluster_vars)
    images = build_deploy_images(force_offline_images, kwargs)
    inclusion = parse_inclusion(kwargs)

    option_args = parse_args(kwargs.get("arg", []))
    if target is not None:
        for arg_name, arg_value in option_args.items():
            kluctl_project.check_dynamic_arg(target, arg_name, arg_value)

    target_args = target.get("args", {}) if target else {}
    seal_args = get_dict_value(target, "sealingConfig.args", {}) if target else {}
    deploy_args = merge_dict(target_args, option_args)
    if for_seal:
        merge_dict(deploy_args, seal_args, False)

    with tempfile.TemporaryDirectory(dir=get_tmp_base_dir()) as tmpdir:
        render_output_dir = kwargs.get("render_output_dir")
        if render_output_dir is None:
            render_output_dir = tmpdir
        d = DeploymentProject(kluctl_project.deployment_dir, jinja_vars, deploy_args, kluctl_project.sealed_secrets_dir)
        c = DeploymentCollection(d, images=images, inclusion=inclusion, tmpdir=render_output_dir, for_seal=for_seal)

        fixed_images = load_fixed_images(kwargs)
        if target is not None:
            for fi in target.get("images", []):
                c.images.add_fixed_image(fi)
        for fi in fixed_images:
            c.images.add_fixed_image(fi)

        if not for_seal:
            c.prepare(k8s_cluster)

        ctx = CommandContext(kluctl_project=kluctl_project, target=target,
                             cluster_vars=cluster_vars, k8s_cluster=k8s_cluster,
                             deployment=d, deployment_collection=c, images=images)
        yield ctx


def build_seen_images(c, detailed):
    ret = []
    for e in c.images.seen_images:
        if detailed:
            a = e
        else:
            a = {
                "image": e["image"],
                "resultImage": e["resultImage"]
            }
        ret.append(a)
    ret.sort(key=lambda x: x["image"])
    return ret
