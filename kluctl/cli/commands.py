import logging
import sys
import tempfile
import time

import click

from kluctl.cli.utils import load_deployment_collection, output_diff_result, build_seen_images, output_yaml_result, \
    output_validate_result
from kluctl.command_error import CommandError
from kluctl.utils.inclusion import Inclusion
from kluctl.utils.k8s_cluster_base import get_k8s_cluster, load_cluster_config
from kluctl.utils.k8s_object_utils import get_long_object_name_from_ref
from kluctl.utils.utils import get_tmp_base_dir, duration

logger = logging.getLogger(__name__)

def deploy_command(obj, kwargs):
    load_cluster_config(obj["cluster_dir"], obj["cluster_name"], dry_run=kwargs["dry_run"])

    k8s_cluster = get_k8s_cluster()

    if not kwargs["yes"] and not kwargs["dry_run"]:
        click.confirm("Do you really want to deploy to the context/cluster %s?" % k8s_cluster.context, err=True, abort=True)

    with load_deployment_collection(kwargs) as (d, c, _):
        parallel = kwargs["parallel"]
        diff_result = c.deploy(k8s_cluster, parallel, kwargs["force_apply"], kwargs["replace_on_error"], kwargs["abort_on_error"])
        deleted_objects = c.find_purge_objects(k8s_cluster)
        output_diff_result(kwargs["output"], c, diff_result, deleted_objects)
        if diff_result.errors:
            sys.exit(1)
        if kwargs["full_diff_after_deploy"]:
            logger.info("Running full diff after deploy")
            c.update_remote_objects_from_diff(diff_result)
            c.inclusion = Inclusion()
            diff_result = c.diff(k8s_cluster, kwargs["force_apply"], False, False, False, False)
            output_diff_result([kwargs["full_diff_after_deploy"]], c, diff_result, deleted_objects)
            if diff_result.errors:
                sys.exit(1)
    sys.exit(0)

def diff_command(obj, kwargs):
    load_cluster_config(obj["cluster_dir"], obj["cluster_name"])

    k8s_cluster = get_k8s_cluster()

    with load_deployment_collection(kwargs) as (d, c, _):
        result = c.diff(k8s_cluster, kwargs["force_apply"], kwargs["replace_on_error"], kwargs["ignore_tags"], kwargs["ignore_labels"], kwargs["ignore_order"])
        deleted_objects = c.find_purge_objects(k8s_cluster)
        output_diff_result(kwargs["output"], c, result, deleted_objects)
    sys.exit(1 if result.errors else 0)

def confirmed_delete_objects(k8s_cluster, objects, kwargs):
    from kluctl.k8s_delete_utils import delete_objects
    if len(objects) == 0:
        return

    click.echo("The following objects will be deleted:", err=True)
    for ref in objects:
        click.echo("  %s" % get_long_object_name_from_ref(ref), err=True)
    if not kwargs["yes"] and not kwargs["dry_run"]:
        click.confirm("Do you really want to delete %d objects?" % len(objects), abort=True, err=True)

    delete_objects(k8s_cluster, objects, False)

def delete_command(obj, kwargs):
    load_cluster_config(obj["cluster_dir"], obj["cluster_name"], dry_run=kwargs["dry_run"])

    k8s_cluster = get_k8s_cluster()

    with load_deployment_collection(kwargs) as (d, c, _):
        objects = c.find_delete_objects(k8s_cluster)
        confirmed_delete_objects(k8s_cluster, objects, kwargs)

def purge_command(obj, kwargs):
    load_cluster_config(obj["cluster_dir"], obj["cluster_name"], dry_run=kwargs["dry_run"])

    k8s_cluster = get_k8s_cluster()

    with load_deployment_collection(kwargs) as (d, c, _):
        objects = c.find_purge_objects(k8s_cluster)
        confirmed_delete_objects(k8s_cluster, objects, kwargs)

def validate_command(obj, kwargs):
    load_cluster_config(obj["cluster_dir"], obj["cluster_name"])

    k8s_cluster = get_k8s_cluster()

    wait_duration = duration(kwargs["wait"]) if kwargs["wait"] else None
    sleep_duration = duration(kwargs["sleep"]) if kwargs["sleep"] else None
    warnings_as_errors = kwargs["warnings_as_errors"]

    start_time = time.time()

    with load_deployment_collection(kwargs, force_offline_images=True) as (d, c, _):
        while True:
            result = c.validate(k8s_cluster)
            failed = len(result.errors) != 0 or (warnings_as_errors and len(result.warnings) != 0)

            output_validate_result(kwargs["output"], result)

            if not failed:
                click.echo("Validation succeeded", file=sys.stderr)
                return

            if wait_duration is None or time.time() - start_time > wait_duration.total_seconds():
                raise CommandError("Validation failed")

            time.sleep(sleep_duration.total_seconds())

            # Need to force re-requesting these objects
            c.forget_remote_objects([x.ref for x in result.errors + result.warnings + result.extra_results])

def render_command(obj, kwargs):
    load_cluster_config(obj["cluster_dir"], obj["cluster_name"], offline=kwargs["offline"])

    logger = logging.getLogger('build')

    output_dir = kwargs["output_dir"]
    if output_dir is None:
        output_dir = tempfile.mkdtemp(dir=get_tmp_base_dir(), prefix="render-")

    with load_deployment_collection(kwargs, output_dir=output_dir) as (d, c, _):
        c.render_deployments()
        logger.info('Rendered into: %s' % output_dir)

    if kwargs["output_images"]:
        result = {
            "images": build_seen_images(c, True)
        }

        output_yaml_result([kwargs["output_images"]], result)

def list_images_command(obj, kwargs):
    cluster_dir = obj["cluster_dir"]
    cluster_name = obj["cluster_name"]
    load_cluster_config(cluster_dir, cluster_name, offline=kwargs["no_kubernetes"])

    with load_deployment_collection(kwargs) as (d, c, images):
        images.raise_on_error = False
        c.render_deployments()

    result = {
        "images": build_seen_images(c, not kwargs["simple"])
    }

    output_yaml_result(kwargs["output"], result)
