import contextlib
import dataclasses
import os
import tempfile

import click

from kluctl.utils.exceptions import CommandError
from kluctl.deployment.deployment_collection import DeploymentCollection
from kluctl.deployment.deployment_project import DeploymentProject
from kluctl.diff.k8s_diff import changes_to_yaml
from kluctl.diff.k8s_pretty_diff import format_diff
from kluctl.image_registries import init_image_registries
from kluctl.deployment.images import Images
from kluctl.utils.external_args import parse_args
from kluctl.utils.inclusion import Inclusion
from kluctl.utils.k8s_cluster_base import get_cluster, get_k8s_cluster
from kluctl.utils.k8s_object_utils import get_long_object_name_from_ref
from kluctl.utils.utils import get_tmp_base_dir
from kluctl.utils.yaml_utils import yaml_load_file, yaml_dump


def build_jinja_vars():
    jinja_vars = {
        'cluster': get_cluster(),
    }

    return jinja_vars

def build_deploy_images(force_offline, kwargs):
    image_registries = None
    if not kwargs.get("no_registries", False):
        image_registries = init_image_registries()
    images = Images(get_k8s_cluster(), image_registries)
    offline = force_offline or kwargs.get("offline", False)
    images.update_images = kwargs.get("update_images", False) and not offline
    images.no_registries = kwargs.get("no_registries", False) or offline
    images.no_kubernetes = kwargs.get("no_kubernetes", False) or offline
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

@contextlib.contextmanager
def load_deployment_collection(kwargs, output_dir=None, force_offline_images=False):
    jinja_vars = build_jinja_vars()
    images = build_deploy_images(force_offline_images, kwargs)
    deploy_args = parse_args(kwargs.get("arg", []))
    sealed_secrets_dir = kwargs.get("sealed_secrets_dir")
    inclusion = parse_inclusion(kwargs)
    with tempfile.TemporaryDirectory(dir=get_tmp_base_dir()) as tmpdir:
        if output_dir is None:
            output_dir = tmpdir
        d = DeploymentProject(kwargs["deployment"], kwargs["deployment_name"], jinja_vars, deploy_args, sealed_secrets_dir)
        c = DeploymentCollection(d, images=images, inclusion=inclusion, tmpdir=output_dir)

        fixed_images = load_fixed_images(kwargs)
        for fi in fixed_images:
            c.seen_images.add_fixed_image(fi)

        yield d, c, images

def build_diff_result(c, deploy_diff_result, deleted_objects, format):
    if format == "diff":
        return format_diff(deploy_diff_result.new_objects, deploy_diff_result.changed_objects, deleted_objects)
    elif format != "yaml":
        raise CommandError(f"Invalid format: {format}")

    result = {
        "diff": changes_to_yaml(deploy_diff_result.new_objects, deploy_diff_result.changed_objects, deploy_diff_result.errors, deploy_diff_result.warnings),
        "images": build_seen_images(c, True),
    }
    if deleted_objects is not None:
        result["deleted"] = [{"ref": dataclasses.asdict(ref)} for ref in deleted_objects]
    return yaml_dump(result)

def build_validate_result(result, format):
    if format == "text":
        str = ""
        if result.warnings:
            str += "Validation Warnings:\n"
            for item in result.warnings:
                str += "  %s: reason=%s, message=%s\n" % (get_long_object_name_from_ref(item.ref), item.reason, item.message)
        if result.errors:
            if str:
                str += "\n"
            str += "Validation Errors:\n"
            for item in result.errors:
                str += "  %s: reason=%s, message=%s\n" % (get_long_object_name_from_ref(item.ref), item.reason, item.message)
        if result.results:
            if str:
                str += "\n"
            str += "Results:\n"
            for item in result.results:
                str += "  %s: reason=%s, message=%s\n" % (get_long_object_name_from_ref(item.ref), item.reason, item.message)
        return str
    if format == "yaml":
        y = yaml_dump(dataclasses.asdict(result))
        return y
    else:
        raise CommandError(f"Invalid format: {format}")

def output_diff_result(output, c, deploy_diff_result, deleted_objects):
    if not output:
        output = ["diff"]
    for o in output:
        s = o.split("=", 1)
        format = s[0]
        path = None
        if len(s) > 1:
            path = s[1]
        s = build_diff_result(c, deploy_diff_result, deleted_objects, format)
        output_result(path, s)

def output_validate_result(output, result):
    if not output:
        output = ["text"]
    for o in output:
        s = o.split("=", 1)
        format = s[0]
        path = None
        if len(s) > 1:
            path = s[1]
        s = build_validate_result(result, format)
        output_result(path, s)

def output_yaml_result(output, result):
    output = output or [None]
    s = yaml_dump(result)
    for o in output:
        output_result(o, s)

def output_result(output_file, result):
    path = None
    if output_file and output_file != "-":
        path = os.path.expanduser(output_file)
    if path is None:
        click.echo(result)
    else:
        with open(path, "wt") as f:
            f.write(result)

def build_seen_images(c, detailed):
    ret = []
    for e in c.seen_images.seen_images:
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
