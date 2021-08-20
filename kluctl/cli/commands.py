import logging
import logging
import os
import sys
import tempfile
import time

import click

from kluctl import get_kluctl_package_dir
from kluctl.cli.utils import output_diff_result, build_seen_images, output_yaml_result, \
    output_validate_result, project_command_context
from kluctl.kluctl_project.kluctl_project import load_kluctl_project_from_args
from kluctl.utils.exceptions import CommandError
from kluctl.utils.inclusion import Inclusion
from kluctl.utils.k8s_object_utils import get_long_object_name_from_ref, ObjectRef
from kluctl.utils.utils import get_tmp_base_dir, duration
from kluctl.utils.yaml_utils import yaml_dump

logger = logging.getLogger(__name__)


def bootstrap_command(obj, kwargs):
    bootstrap_path = os.path.join(get_kluctl_package_dir(), "bootstrap")

    kwargs["local_deployment"] = bootstrap_path
    kwargs["full_diff_after_deploy"] = False
    with project_command_context(kwargs) as cmd_ctx:
        existing, warnings = cmd_ctx.k8s_cluster.get_single_object(ObjectRef("apiextensions.k8s.io/v1", "CustomResourceDefinition", "sealedsecrets.bitnami.com"))
        if existing:
            if not kwargs["yes"] and existing["metadata"].get("labels", {}).get("kluctl.io/component") != "bootstrap":
                click.confirm("It looks like you're trying to bootstrap a cluster that already has the sealed-secrets "
                              "deployed but not managed by kluctl. Do you really want to continue bootrapping?",
                              err=True, abort=True)

        deploy_command2(obj, kwargs, cmd_ctx)
        purge_command2(obj, kwargs, cmd_ctx)

def deploy_command(obj, kwargs):
    with project_command_context(kwargs) as cmd_ctx:
        deploy_command2(obj, kwargs, cmd_ctx)

def deploy_command2(obj, kwargs, cmd_ctx):
    if not kwargs["yes"] and not kwargs["dry_run"]:
        click.confirm("Do you really want to deploy to the context/cluster %s?" % cmd_ctx.k8s_cluster.context, err=True, abort=True)

    parallel = kwargs["parallel"]
    diff_result = cmd_ctx.deployment_collection.deploy(cmd_ctx.k8s_cluster, parallel, kwargs["force_apply"], kwargs["replace_on_error"], kwargs["abort_on_error"])
    deleted_objects = cmd_ctx.deployment_collection.find_purge_objects(cmd_ctx.k8s_cluster)
    output_diff_result(kwargs["output"], cmd_ctx.deployment_collection, diff_result, deleted_objects)
    if diff_result.errors:
        sys.exit(1)
    if kwargs["full_diff_after_deploy"]:
        logger.info("Running full diff after deploy")
        cmd_ctx.deployment_collection.update_remote_objects_from_diff(diff_result)
        cmd_ctx.deployment_collection.inclusion = Inclusion()
        diff_result = cmd_ctx.deployment_collection.diff(cmd_ctx.k8s_cluster, kwargs["force_apply"], False, False, False, False)
        output_diff_result([kwargs["full_diff_after_deploy"]], cmd_ctx.deployment_collection, diff_result, deleted_objects)
        if diff_result.errors:
            sys.exit(1)

def diff_command(obj, kwargs):
    with project_command_context(kwargs) as cmd_ctx:
        result = cmd_ctx.deployment_collection.diff(cmd_ctx.k8s_cluster, kwargs["force_apply"], kwargs["replace_on_error"], kwargs["ignore_tags"], kwargs["ignore_labels"], kwargs["ignore_order"])
        deleted_objects = cmd_ctx.deployment_collection.find_purge_objects(cmd_ctx.k8s_cluster)
        output_diff_result(kwargs["output"], cmd_ctx.deployment_collection, result, deleted_objects)
    sys.exit(1 if result.errors else 0)

def confirmed_delete_objects(k8s_cluster, objects, kwargs):
    from kluctl.utils.k8s_delete_utils import delete_objects
    if len(objects) == 0:
        return

    click.echo("The following objects will be deleted:", err=True)
    for ref in objects:
        click.echo("  %s" % get_long_object_name_from_ref(ref), err=True)
    if not kwargs["yes"] and not kwargs["dry_run"]:
        click.confirm("Do you really want to delete %d objects?" % len(objects), abort=True, err=True)

    delete_objects(k8s_cluster, objects, False)

def delete_command(obj, kwargs):
    with project_command_context(kwargs) as cmd_ctx:
        objects = cmd_ctx.deployment_collection.find_delete_objects(cmd_ctx.k8s_cluster)
        confirmed_delete_objects(cmd_ctx.k8s_cluster, objects, kwargs)

def purge_command(obj, kwargs):
    with project_command_context(kwargs) as cmd_ctx:
        purge_command2(obj, kwargs, cmd_ctx)

def purge_command2(obj, kwargs, cmd_ctx):
    objects = cmd_ctx.deployment_collection.find_purge_objects(cmd_ctx.k8s_cluster)
    confirmed_delete_objects(cmd_ctx.k8s_cluster, objects, kwargs)

def validate_command(obj, kwargs):
    wait_duration = duration(kwargs["wait"]) if kwargs["wait"] else None
    sleep_duration = duration(kwargs["sleep"]) if kwargs["sleep"] else None
    warnings_as_errors = kwargs["warnings_as_errors"]

    start_time = time.time()

    with project_command_context(kwargs, force_offline_images=True) as cmd_ctx:
        while True:
            result = cmd_ctx.deployment_collection.validate(cmd_ctx.k8s_cluster)
            failed = len(result.errors) != 0 or (warnings_as_errors and len(result.warnings) != 0)

            output_validate_result(kwargs["output"], result)

            if not failed:
                click.echo("Validation succeeded", file=sys.stderr)
                return

            if wait_duration is None or time.time() - start_time > wait_duration.total_seconds():
                raise CommandError("Validation failed")

            time.sleep(sleep_duration.total_seconds())

            # Need to force re-requesting these objects
            cmd_ctx.deployment_collection.forget_remote_objects([x.ref for x in result.errors + result.warnings + result.extra_results])

def render_command(obj, kwargs):
    logger = logging.getLogger('build')

    output_dir = kwargs["output_dir"]
    if output_dir is None:
        output_dir = tempfile.mkdtemp(dir=get_tmp_base_dir(), prefix="render-")

    with project_command_context(kwargs, output_dir=output_dir) as cmd_ctx:
        cmd_ctx.deployment_collection.render_deployments()
        logger.info('Rendered into: %s' % output_dir)

        if kwargs["output_images"]:
            result = {
                "images": build_seen_images(cmd_ctx.deployment_collection, True)
            }

            output_yaml_result([kwargs["output_images"]], result)

def list_images_command(obj, kwargs):
    with project_command_context(kwargs, force_offline_kubernetes=kwargs["no_kubernetes"]) as cmd_ctx:
        cmd_ctx.images.raise_on_error = False
        cmd_ctx.deployment_collection.render_deployments()

        result = {
            "images": build_seen_images(cmd_ctx.deployment_collection, not kwargs["simple"])
        }

        output_yaml_result(kwargs["output"], result)

def list_targets_command(obj, kwargs):
    with load_kluctl_project_from_args(kwargs) as kluctl_project:
        kluctl_project.load(True)
        kluctl_project.load_targets()

        result = {
            "involved_repos": kluctl_project.involved_repos,
            "targets": kluctl_project.targets,
        }

        output_yaml_result(kwargs["output"], result)
