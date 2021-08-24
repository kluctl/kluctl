import itertools
import logging
import os
import re
import sys

import click
from git import Git

from kluctl.cli.commands import output_yaml_result
from kluctl.cli.utils import project_command_context
from kluctl.deployment.helm_chart import HelmChart
from kluctl.image_registries import init_image_registries
from kluctl.utils.k8s_object_utils import get_long_object_name_from_ref
from kluctl.utils.pretty_table import pretty_table
from kluctl.utils.utils import MyThreadPoolExecutor
from kluctl.utils.versions import LooseVersionComparator

logger = logging.getLogger(__name__)

def list_images_helper(images):
    registries = init_image_registries()
    if len(registries) == 0:
        logger.error("No registry configured, aborting")
        sys.exit(1)

    def do_list(image):
        for r in registries:
            if r.is_image_from_registry(image):
                try:
                    tags = r.list_tags_for_image(image)
                    return tags
                except Exception as e:
                    return e
        return Exception(f"No registry found for image {image}, aborting")

    with MyThreadPoolExecutor(max_workers=8) as executor:
        futures = {}
        for image in images:
            futures[image] = executor.submit(do_list, image)
        result = {}
        for image in images:
            result[image] = futures[image].result()
        return result

def list_image_tags_command(image, output):
    images = set(image)
    tags = list_images_helper(images)

    result = {
        "images": {}
    }
    for image in images:
        t = tags[image]
        if isinstance(t, Exception):
            result["images"][image] = {
                "error": str(t),
            }
        else:
            result["images"][image] = {
                "tags": t,
            }

    output_yaml_result(output, result)

def helm_pull_command(kwargs):
    for dirpath, dirnames, filenames in os.walk(kwargs["local_deployment"]):
        for fname in filenames:
            if fname == 'helm-chart.yml':
                path = os.path.join(dirpath, fname)
                logger.info("Pulling for %s" % path)
                chart = HelmChart(path)
                chart.pull()

def helm_update_command(upgrade, commit, kwargs):
    for dirpath, dirnames, filenames in os.walk(kwargs["local_deployment"]):
        for fname in filenames:
            if fname == 'helm-chart.yml':
                path = os.path.join(dirpath, fname)
                chart = HelmChart(path)
                new_version = chart.check_update()
                if new_version is None:
                    continue
                logger.info("Chart %s has new version %s available. Old version is %s." % (path, new_version, chart.conf["chartVersion"]))

                if upgrade:
                    if chart.conf.get("skipUpdate", False):
                        logger.info("NOT upgrading chart %s as skipUpdate was set to true" % path)
                        continue

                    old_version = chart.conf["chartVersion"]
                    chart.conf["chartVersion"] = new_version
                    chart.save(path)
                    logger.info("Pulling for %s" % path)
                    chart.pull()

                    if commit:
                        msg = "Updated helm chart %s from %s to %s" % (dirpath, old_version, new_version)
                        logger.info("Committing: %s" % msg)
                        g = Git(working_dir=kwargs["deployment"])
                        g.add(os.path.join(dirpath, "charts"))
                        g.add(path)
                        g.commit("-m", msg, os.path.join(dirpath, "charts"), path)

def check_image_updates(obj, kwargs):
    with project_command_context(kwargs) as cmd_ctx:
        cmd_ctx.images.no_kubernetes = True
        cmd_ctx.images.raise_on_error = False
        cmd_ctx.deployment_collection.render_deployments()
        cmd_ctx.deployment_collection.build_kustomize_objects()
        rendered_images = cmd_ctx.deployment_collection.find_rendered_images()

    prefix_pattern = re.compile(r"^([a-zA-Z]+[a-zA-Z-_.]*)")
    suffix_pattern = re.compile(r"([-][a-zA-Z]+[a-zA-Z-_.]*)$")

    all_repos = set(itertools.chain.from_iterable(rendered_images.values()))
    all_repos = set(x.split(":", 1)[0] for x in all_repos if ":" in x)
    tags = list_images_helper(all_repos)

    table = [["OBJECT", "IMAGE", "OLD", "NEW"]]

    for ref, images in rendered_images.items():
        for image in images:
            obj_name = get_long_object_name_from_ref(ref)
            if ":" not in image:
                logger.warning("%s: Ignoring image %s as it doesn't specify a tag'" % (obj_name, image))
                continue
            repo, cur_tag = tuple(image.split(":", 1))
            repo_tags = tags[repo]
            if isinstance(repo_tags, Exception):
                logger.warning("%s: Failed to list tags for %s. %s" % (obj_name, repo, str(repo_tags)))
                continue

            prefix = prefix_pattern.match(cur_tag)
            suffix = suffix_pattern.search(cur_tag)
            prefix = prefix.group(1) if prefix else ""
            suffix = suffix.group(1) if suffix else ""
            has_dot = "." in cur_tag
            def do_filter(x):
                if has_dot != ("." in x):
                    return False
                if prefix and not x.startswith(prefix):
                    return False
                if suffix and not x.endswith(suffix):
                    return False
                return True
            def do_key(x):
                if prefix:
                    x = x[len(prefix):]
                if suffix:
                    x = x[:-len(suffix)]
                return LooseVersionComparator(x)
            filtered_tags = [x for x in repo_tags if do_filter(x)]
            filtered_tags.sort(key=lambda x: do_key(x))
            latest_tag = filtered_tags[-1]

            if latest_tag != cur_tag:
                table.append([obj_name, repo, cur_tag, latest_tag])

    table_txt = pretty_table(table, [60])
    click.echo(table_txt)
