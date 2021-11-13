import logging
import os
import sys
import tempfile
import time

import click

from kluctl import get_kluctl_package_dir
from kluctl.cli.utils import build_seen_images, project_command_context
from kluctl.cli.command_result import output_command_result, output_validate_result, output_yaml_result
from kluctl.kluctl_project.kluctl_project import load_kluctl_project_from_args
from kluctl.utils.dict_utils import get_dict_value
from kluctl.utils.exceptions import CommandError
from kluctl.utils.k8s_object_utils import get_long_object_name_from_ref, ObjectRef
from kluctl.utils.utils import get_tmp_base_dir, duration

logger = logging.getLogger(__name__)


def bootstrap_command(obj, kwargs):
    bootstrap_path = os.path.join(get_kluctl_package_dir(), "bootstrap")

    if not kwargs.get("local_deployment"):
        kwargs["local_deployment"] = bootstrap_path
    with project_command_context(kwargs) as cmd_ctx:
        existing, warnings = cmd_ctx.k8s_cluster.get_single_object(ObjectRef("apiextensions.k8s.io/v1", "CustomResourceDefinition", "sealedsecrets.bitnami.com"))
        if existing:
            if not kwargs["yes"] and get_dict_value(existing, 'metadata.labels."kluctl.io/component"') != "bootstrap":
                click.confirm("It looks like you're trying to bootstrap a cluster that already has the sealed-secrets "
                              "deployed but not managed by kluctl. Do you really want to continue bootstrapping?",
                              err=True, abort=True)

        deploy_command2(obj, kwargs, cmd_ctx)
        prune_command2(obj, kwargs, cmd_ctx)

def deploy_command(obj, kwargs):
    with project_command_context(kwargs) as cmd_ctx:
        deploy_command2(obj, kwargs, cmd_ctx)

def deploy_command2(obj, kwargs, cmd_ctx):
    if not kwargs["yes"] and not kwargs["dry_run"]:
        click.confirm("Do you really want to deploy to the context/cluster %s?" % cmd_ctx.k8s_cluster.context, err=True, abort=True)

    result = cmd_ctx.deployment_collection.deploy(cmd_ctx.k8s_cluster, kwargs["force_apply"],
                                                       kwargs["replace_on_error"], kwargs["force_replace_on_error"], kwargs["abort_on_error"])
    output_command_result(kwargs["output"], cmd_ctx.deployment_collection, result)
    if result.errors:
        sys.exit(1)

def diff_command(obj, kwargs):
    with project_command_context(kwargs) as cmd_ctx:
        result = cmd_ctx.deployment_collection.diff(
            cmd_ctx.k8s_cluster, kwargs["force_apply"], kwargs["replace_on_error"], kwargs["force_replace_on_error"],
            kwargs["ignore_tags"], kwargs["ignore_labels"], kwargs["ignore_annotations"], kwargs["ignore_order"])
        output_command_result(kwargs["output"], cmd_ctx.deployment_collection, result)
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

def prune_command(obj, kwargs):
    with project_command_context(kwargs) as cmd_ctx:
        prune_command2(obj, kwargs, cmd_ctx)

def prune_command2(obj, kwargs, cmd_ctx):
    objects = cmd_ctx.deployment_collection.find_orphan_objects(cmd_ctx.k8s_cluster)
    confirmed_delete_objects(cmd_ctx.k8s_cluster, objects, kwargs)

def poke_images_command(obj, kwargs):
    with project_command_context(kwargs) as cmd_ctx:
        if not kwargs["yes"] and not kwargs["dry_run"]:
            click.confirm("Do you really want to poke images to the context/cluster %s?" % cmd_ctx.k8s_cluster.context,
                          err=True, abort=True)
        result = cmd_ctx.deployment_collection.poke_images(cmd_ctx.k8s_cluster)
        output_command_result(kwargs["output"], cmd_ctx.deployment_collection, result)
        if result.errors:
            sys.exit(1)

def downscale_command(obj, kwargs):
    with project_command_context(kwargs) as cmd_ctx:
        if not kwargs["yes"] and not kwargs["dry_run"]:
            click.confirm("Do you really want to downscale on context/cluster %s?" % cmd_ctx.k8s_cluster.context,
                          err=True, abort=True)
        result = cmd_ctx.deployment_collection.downscale(cmd_ctx.k8s_cluster)
        output_command_result(kwargs["output"], cmd_ctx.deployment_collection, result)
        if result.errors:
            sys.exit(1)

def validate_command(obj, kwargs):
    wait_duration = duration(kwargs["wait"]) if kwargs["wait"] else None
    sleep_duration = duration(kwargs["sleep"]) if kwargs["sleep"] else None
    warnings_as_errors = kwargs["warnings_as_errors"]

    start_time = time.time()

    with project_command_context(kwargs) as cmd_ctx:
        while True:
            result = cmd_ctx.deployment_collection.validate()
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

    if kwargs["render_output_dir"] is None:
        kwargs["render_output_dir"] = tempfile.mkdtemp(dir=get_tmp_base_dir(), prefix="render-")

    with project_command_context(kwargs, force_offline_images=kwargs["offline"], force_offline_kubernetes=kwargs["offline"]) as cmd_ctx:
        logger.info('Rendered into: %s' % cmd_ctx.deployment_collection.tmpdir)

        if kwargs["output_images"]:
            result = {
                "images": build_seen_images(cmd_ctx.deployment_collection, True)
            }

            output_yaml_result([kwargs["output_images"]], result)

        if kwargs["output_single_yaml"]:
            all_yamls = []
            for d in cmd_ctx.deployment_collection.deployments:
                all_yamls += d.objects
            output_yaml_result([kwargs["output_single_yaml"]], all_yamls, all=True)

def list_images_command(obj, kwargs):
    with project_command_context(kwargs, force_offline_kubernetes=kwargs["no_kubernetes"]) as cmd_ctx:
        cmd_ctx.images.raise_on_error = False

        result = {
            "images": build_seen_images(cmd_ctx.deployment_collection, not kwargs["simple"])
        }

        output_yaml_result(kwargs["output"], result)

def list_targets_command(obj, kwargs):
    with load_kluctl_project_from_args(kwargs) as kluctl_project:
        result = {
            "involved_repos": kluctl_project.involved_repos,
            "targets": kluctl_project.targets,
        }

        output_yaml_result(kwargs["output"], result)

def archive_command(obj, kwargs):
    with load_kluctl_project_from_args(kwargs) as kluctl_project:
        kluctl_project.create_tgz(kwargs["output_archive"], kwargs["output_metadata"], kwargs["reproducible"])
