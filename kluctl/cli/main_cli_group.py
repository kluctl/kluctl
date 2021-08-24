import datetime
import logging
import os.path
import sys
from distutils.version import LooseVersion

import click
from click_option_group import optgroup

from kluctl import _version
from kluctl.utils.external_tools import get_external_tool_path
from kluctl.utils.gitlab.gitlab_util import init_gitlab_util_from_glab
from kluctl.utils.utils import get_tmp_base_dir
from kluctl.utils.yaml_utils import yaml_load_file, yaml_save_file

logger = logging.getLogger(__name__)

LATEST_RELEASE_URL = "https://api.github.com/repos/codablock/kluctl/releases/latest"
HELP_BASE_URL = "https://github.com/codablock/kluctl/blob/%s/docs" % (("v%s" % _version.__version__) if _version.__version__ != "0.0.0" else "main")

def build_help_link(md, section=None):
    s = "%s/%s" % (HELP_BASE_URL, md)
    if section:
        s = "%s#%s" % (s, section)
    return s

def setup_logging(verbose):
    level = logging.INFO
    if verbose:
        level = logging.DEBUG

    format = '%(asctime)s %(name)-30s %(levelname)-8s %(message)s'

    logging.basicConfig(level=level, format=format)

    logging.getLogger('urllib3').setLevel(logging.WARN)
    logging.getLogger('urllib3.connectionpool').setLevel(logging.ERROR)
    logging.getLogger('filelock').setLevel(logging.WARN)

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
        r = requests.get(LATEST_RELEASE_URL, timeout=5)
        r.raise_for_status()
        release = r.json()
        latest_version = release["tag_name"]
        if latest_version.startswith("v"):
            latest_version = latest_version[1:]
    except Exception as e:
        logger.warning("Failed to query latest kluctl version. %s" % str(e))
        return
    local_version = LooseVersion(_version.__version__)
    if local_version < latest_version:
        logger.warning("You are using an outdated version (%s) of kluctl. You should update soon to version %s",
                       str(local_version), str(latest_version))

def check_external_tools_installed():
    get_external_tool_path("kustomize")
    get_external_tool_path("helm")
    get_external_tool_path("kubeseal")

def configure(ctx, param, filename):
    if not os.path.exists(filename):
        return
    config = yaml_load_file(filename)
    config = config["config"]
    ctx.default_map = config

@click.group(context_settings={"auto_envvar_prefix": "kluctl", "max_content_width": 200})
@click.version_option(version=_version.__version__, package_name="kluctl")
@click.option("--verbose", "-v", help="Enable verbose logging", default=False, is_flag=True)
@click.option("--no-update-check", help="Disable update check on startup", default=False, is_flag=True)
@click.option(
    '--config',
    type=click.Path(dir_okay=False),
    default=os.path.join(click.get_app_dir("kluctl", force_posix=True), "config.yml"),
    callback=configure,
    is_eager=True,
    expose_value=False,
    help='Read option defaults from the specified config file',
    show_default=True
)
@click.pass_context
def cli_group(ctx: click.Context, verbose, no_update_check):
    ctx.ensure_object(dict)
    obj = ctx.obj
    obj["verbose"] = verbose
    setup_logging(verbose)
    if not no_update_check:
        check_new_version()
    if "--help" not in sys.argv:
        check_external_tools_installed()
        init_gitlab_util_from_glab()

def wrapper_helper(options):
    def wrapper(func):
        for o in reversed(options):
            func = o(func)
        return func
    return wrapper

def misc_arguments(yes=False, dry_run=False, force_apply=False, replace_on_error=False,
                   ignore_labels=False, ignore_order=False, abort_on_error=False, output_format=False, output=False):
    options = []

    options.append(optgroup.group("Misc arguments"))
    if yes:
        options.append(optgroup.option("-y", "--yes",
                                       help="Supresses 'Are you sure?' questions and proceeds as if you would "
                                            "answer 'yes'.",
                                       default=False, is_flag=True))
    if dry_run:
        options.append(optgroup.option("--dry-run",
                                       help="Performs all kubernetes API calls in dry-run mode.",
                                       default=False, is_flag=True))
    if force_apply:
        options.append(optgroup.option("--force-apply",
                                       help="Force conflict resolution when applying. See documentation for details",
                                       default=False, is_flag=True))
    if replace_on_error:
        options.append(optgroup.option("--replace-on-error",
                                       help="When patching an object fails, try to delete it and then retry. "
                                            "See documentation for more details.",
                                       default=False, is_flag=True))
    if ignore_labels:
        options.append(optgroup.option("--ignore-tags",
                                       help="Ignores changes in tags when diffing",
                                       default=False, is_flag=True))
        options.append(optgroup.option("--ignore-labels",
                                       help="Ignores changes in labels when diffing",
                                       default=False, is_flag=True))
        options.append(optgroup.option("--ignore-annotations",
                                       help="Ignores changes in annotations when diffing",
                                       default=False, is_flag=True))
    if ignore_order:
        options.append(optgroup.option("--ignore-order",
                                       help="Ignores changes in order when diffing",
                                       default=False, is_flag=True))
    if abort_on_error:
        options.append(optgroup.option("--abort-on-error",
                                       help="Abort deploying when an error occurs instead of trying the remaining "
                                            "deployments",
                                       default=False, is_flag=True))
    if output_format:
        options.append(optgroup.option("-o", "--output",
                                       help="Specify output format and target file, in the format 'format=path'. "
                                            "Format can either be 'diff' or 'yaml'. Can be specified multiple times. "
                                            "The actual format for yaml is currently not documented and subject to "
                                            "change.",
                                       multiple=True))
    if output:
        options.append(optgroup.option("-o", "--output",
                                       help="Specify output target file. Can be specified multiple times",
                                       multiple=True))
    return wrapper_helper(options)

def kluctl_project_args(with_d=True, with_a=True, with_t=True):
    options = []
    options.append(optgroup.group("Project arguments",
                                  help="Define where and how to load the kluctl project and its components from."))
    if with_d:
        options.append(optgroup.option("--project-url", "-p",
                                       help="Git url of the kluctl project. If not specified, the current directory "
                                            "will be used instead of a remote Git project"))
        options.append(optgroup.option("--project-ref", "-b",
                                       help="Git ref of the kluctl project. Only used when --project-url was given."))
        options.append(optgroup.option("--project-config", "-c",
                                       help="Location of the .kluctl.yml config file. Defaults to $PROJECT/.kluctl.yml",
                                       type=click.Path(dir_okay=False)))
        options.append(optgroup.option("--local-clusters",
                                       help="Local clusters directory. Overrides the project from .kluctl.yml",
                                       type=click.Path(file_okay=False)))
        options.append(optgroup.option("--local-deployment",
                                       help="Local deployment directory. Overrides the project from .kluctl.yml",
                                       type=click.Path(file_okay=False)))
        options.append(optgroup.option("--local-sealed-secrets",
                                       help="Local sealed-secrets directory. Overrides the project from .kluctl.yml",
                                       type=click.Path(file_okay=False)))
        options.append(optgroup.option("--from-archive",
                                       help="Load project (.kluctl.yml, cluster, ...) from archive. Given path can "
                                            "either be an archive file or a directory with the extracted contents.",
                                       type=click.Path(dir_okay=True, file_okay=True)))
        options.append(optgroup.option("--from-archive-metadata",
                                       help="Specify where to load metadata (targets, ...) from. If not specified, "
                                            "metadata is assumed to be part of the archive.",
                                       type=click.Path(dir_okay=True, file_okay=True)))
        options.append(optgroup.option("--deployment-name",
                                       help="Name of the kluctl deployment. Used when resolving sealed-secrets. "
                                            "Defaults to the base name of --local-deployment/--project-url"))
        options.append(optgroup.option("--cluster",
                                       help="Specify/Override cluster"))
    if with_a:
        options.append(optgroup.option("-a", "--arg",
                                       help="Template argument in the form name=value",
                                       multiple=True))
    if with_t:
        options.append(optgroup.option("-t", "--target",
                                       help="Target name to run command for. Target must exist in .kluctl.yml."))
    return wrapper_helper(options)

def include_exclude_args():
    options = []
    options.append(optgroup.group("Inclusion/Exclusion arguments", help="Control inclusion/exclusion."))
    options.append(optgroup.option("-I", "--include-tag",
                                   help="Include deployments with given tag.",
                                   multiple=True))
    options.append(optgroup.option("-E", "--exclude-tag",
                                   help="Exclude deployments with given tag. Exclusion has precedence over inclusion, "
                                        "meaning that explicitly excluded deployments will always be excluded even if "
                                        "an inclusion rule would match the same deployment.",
                                   multiple=True))
    options.append(optgroup.option("--include-kustomize-dir",
                                   help="Include kustomize dir. The path must be relative to the root deployment "
                                        "project.",
                                   multiple=True))
    options.append(optgroup.option("--exclude-kustomize-dir",
                                   help="Exclude kustomize dir. The path must be relative to the root deployment "
                                        "project. Exclusion has precedence over inclusion, same as in "
                                        "--exclude-tag",
                                   multiple=True))
    return wrapper_helper(options)

def image_args():
    options = []
    options.append(optgroup.group("Image arguments"))
    options.append(optgroup.option("-F", "--fixed-image",
                                   help="Pin an image to a given version. "
                                        "Expects '--fixed-image=image<:namespace:deployment:container>=result'",
                                   multiple=True))
    options.append(optgroup.option("--fixed-images-file",
                                   help="Use .yml file to pin image versions. "
                                        "See output of list-images sub-command or read the "
                                        "documentation for details about the output format",
                                   required=False, type=click.Path(dir_okay=False)))
    options.append(optgroup.option("-u", "--update-images",
                                   help="This causes kluctl to prefer the latest image found in registries, "
                                        "based on the `latest_image` filters provided to 'images.get_image(...)' calls. "
                                        "Use this flag if you want to update to the latest versions/tags of all images. "
                                        "'-u' takes precedence over `--fixed-image/--fixed-images-file`, meaning that "
                                        "the latest images are used even if an older image is given via fixed images.",
                                   default=False, is_flag=True))
    return wrapper_helper(options)
