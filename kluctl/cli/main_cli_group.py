import datetime
import logging
import os.path
from distutils.version import LooseVersion

import click
from click_option_group import optgroup

from kluctl import _version
from kluctl.utils.utils import get_tmp_base_dir
from kluctl.utils.yaml_utils import yaml_load_file, yaml_save_file

logger = logging.getLogger(__name__)

VERSIONS_URL = os.environ.get("VERSIONS_URL", "https://agent-ralf.tools.hero-t1.hellmann.net/api/v1/versions/kluctl")

def setup_logging(verbose):
    level = logging.INFO
    if verbose:
        level = logging.DEBUG

    format = '%(asctime)s %(name)-30s %(levelname)-8s %(message)s'

    logging.basicConfig(level=level, format=format)

    logging.getLogger('urllib3').setLevel(logging.WARN)
    logging.getLogger('urllib3.connectionpool').setLevel(logging.ERROR)

def check_new_version():
    if _version.__version__ == "0.0.0":
        # local dev version
        return

    version_check_path = os.path.join(get_tmp_base_dir(), "version_check.yml")
    try:
        y = yaml_load_file(version_check_path)
        now = datetime.datetime.utcnow()
        check_time = datetime.datetime.fromisoformat(y["last_version_check"]) + datetime.timedelta(hours=1)
        if now < check_time:
            return
    except:
        pass

    y = {
        "last_version_check": str(datetime.datetime.utcnow()),
    }
    yaml_save_file(y, version_check_path)

    logger.debug("Checking for new kluctl version")
    try:
        import requests
        r = requests.get(VERSIONS_URL, timeout=5)
        r.raise_for_status()
        versions = r.json()
    except Exception as e:
        logger.warning("Failed to query latest kluctl version. %s" % str(e))
        return
    versions = [LooseVersion(v) for v in versions]
    versions.sort()
    latest_version = versions[-1]
    local_version = LooseVersion(_version.__version__)
    if local_version < latest_version:
        logger.warning("You are using an outdated version (%s) of kluctl. You should update soon to version %s",
                       str(local_version), str(latest_version))

@click.group(context_settings={"auto_envvar_prefix": "kluctl"})
@click.version_option(version=_version.__version__, package_name="kluctl")
@optgroup.group("Common options")
@optgroup.option("--verbose", "-v", help="Enable verbose logging", default=False, is_flag=True)
@optgroup.option("--no-update-check", help="Disable update check on startup", default=False, is_flag=True)
@optgroup.group("Cluster specific options")
@optgroup.option("--cluster-dir", "-C", help="Directory that contains cluster configurations. Defaults to $PWD/clusters", default='clusters', type=click.Path(file_okay=False))
@optgroup.option("--cluster-name", "-N", help="Name of the target cluster", required=False)
@click.pass_context
def cli_group(ctx: click.Context, verbose, cluster_dir, cluster_name, no_update_check):
    ctx.ensure_object(dict)
    obj = ctx.obj
    obj["verbose"] = verbose
    obj["cluster_dir"] = cluster_dir
    obj["cluster_name"] = cluster_name
    setup_logging(verbose)
    if not no_update_check:
        check_new_version()

def wrapper_helper(options):
    def wrapper(func):
        for o in reversed(options):
            func = o(func)
        return func
    return wrapper

def misc_arguments(yes=False, dry_run=False, parallel=False, force_apply=False, replace_on_error=False,
                   ignore_labels=False, ignore_order=False, abort_on_error=False, output_format=False, output=False):
    options = []

    options.append(optgroup.group("Misc arguments"))
    if yes:
        options.append(optgroup.option("-y", "--yes", help="Answer yes on questions", default=False, is_flag=True))
    if dry_run:
        options.append(optgroup.option("--dry-run", help="Dry run", default=False, is_flag=True))
    if parallel:
        options.append(optgroup.option("--parallel", help="Allow apply objects in parallel", default=True, is_flag=True))
    if force_apply:
        options.append(optgroup.option("--force-apply", help="Force conflict resolution when applying", default=False, is_flag=True))
    if replace_on_error:
        options.append(optgroup.option("--replace-on-error", help="When patching an object fails, try to delete it and then retry", default=False, is_flag=True))
    if ignore_labels:
        options.append(optgroup.option("--ignore-tags", help="Ignores changes in tags when diffing", default=False, is_flag=True))
        options.append(optgroup.option("--ignore-labels", help="Ignores changes in labels when diffing", default=False, is_flag=True))
    if ignore_order:
        options.append(optgroup.option("--ignore-order", help="Ignores changes in order when diffing", default=False, is_flag=True))
    if abort_on_error:
        options.append(optgroup.option("--abort-on-error", help="Abort deploying when an error occurs instead of trying the remaining deployments", default=False, is_flag=True))
    if output_format:
        options.append(optgroup.option("-o", "--output",
                                       help="Specify output format and target file, in the format 'format=path'. "
                                            "Format can either be 'diff' or 'yaml'. Can be specified multiple times",
                                       multiple=True))
    if output:
        options.append(optgroup.option("-o", "--output",
                                       help="Specify output target file. Can be specified multiple times",
                                       multiple=True))
    return wrapper_helper(options)

def project_args(with_d=True, with_a=True):
    options = []
    options.append(optgroup.group("Project arguments"))
    if with_d:
        options.append(optgroup.option("-d", "--deployment", help="Deployment directory", required=False, default=".", type=click.Path(file_okay=False)))
        options.append(optgroup.option("--deployment-name", help="Name of the deployment. Used when resolving sealed-secrets. Defaults to the base name of --deployment"))
    if with_a:
        options.append(optgroup.option("-a", "--arg", help="Template argument", multiple=True))
    options.append(optgroup.option("--sealed-secrets-dir", help="Sealed secrets directory (default is $PWD/.sealed-secrets)", default='.sealed-secrets', type=click.Path(file_okay=False)))
    return wrapper_helper(options)

def include_exclude_args():
    options = []
    options.append(optgroup.group("Inclusion/Exclusion arguments"))
    options.append(optgroup.option("-I", "--include-tag", help="Include deployments with given tag", multiple=True))
    options.append(optgroup.option("-E", "--exclude-tag", help="Exclude deployments with given tag", multiple=True))
    options.append(optgroup.option("--include-kustomize-dir", help="Include kustomize dir", multiple=True))
    options.append(optgroup.option("--exclude-kustomize-dir", help="Exclude kustomize dir", multiple=True))
    return wrapper_helper(options)

def image_args():
    options = []
    options.append(optgroup.group("Image arguments"))
    options.append(optgroup.option("-F", "--fixed-image", help="Pin an image to a given version. Expects '--fixed-image=image<:namespace:deployment:container>=result'", multiple=True))
    options.append(optgroup.option("--fixed-images-file", help="Use .yml file to pin image versions. See output of list-images sub-command", required=False, type=click.Path(dir_okay=False)))
    options.append(optgroup.option("-u", "--update-images", help="Update images to latest version found in the image registries", default=False, is_flag=True))
    return wrapper_helper(options)
