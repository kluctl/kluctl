import click
from click_option_group import optgroup

from kluctl.cli.main_cli_group import cli_group, kluctl_project_args, misc_arguments, image_args, include_exclude_args

@cli_group.command("bootstrap",
                   help="Bootstrap a target cluster.\n\n"
                        "This will install the sealed-secrets operator into the specified cluster if not already "
                        "installed.\n\n"
                        "Either --target or --cluster must be specified.")
@kluctl_project_args(with_a=False)
@misc_arguments(yes=True, dry_run=True, force_apply=True, replace_on_error=True, hook_timeout=True, abort_on_error=True, output_format=True)
@click.pass_obj
def bootstrap_command_stub(obj, **kwargs):
    from kluctl.cli.commands import bootstrap_command
    bootstrap_command(obj, kwargs)

@cli_group.command("deploy",
                   help="Deploys a target to the corresponding cluster.\n\n"
                        "This command will also output a diff between the initial state and the state after "
                        "deployment. The format of this diff is the same as for the `diff` command. "
                        "It will also output a list of prunable objects (without actually deleting them).")
@kluctl_project_args()
@image_args()
@include_exclude_args()
@misc_arguments(yes=True, dry_run=True, force_apply=True, replace_on_error=True, abort_on_error=True, hook_timeout=True, output_format=True, render_output_dir=True)
@click.pass_obj
def deploy_command_stub(obj, **kwargs):
    from kluctl.cli.commands import deploy_command
    deploy_command(obj, kwargs)

@cli_group.command("diff",
                   help="Perform a diff between the locally rendered target and the already deployed target.\n\n"
                        "The output is by default in human readable form (a table combined with unified diffs). "
                        "The output can also be changed to output yaml file. Please note however that the format "
                        "is currently not documented and prone to changes.\n\n"
                        "After the diff is performed, the command will also search for prunable objects and list them.")
@kluctl_project_args()
@image_args()
@include_exclude_args()
@misc_arguments(force_apply=True, replace_on_error=True, ignore_labels=True, ignore_order=True, output_format=True, render_output_dir=True)
@click.pass_obj
def diff_command_stub(obj, **kwargs):
    from kluctl.cli.commands import diff_command
    diff_command(obj, kwargs)

@cli_group.command("delete",
                   help="Delete the a target (or parts of it) from the corresponding cluster.\n\n"
                        "Objects are located based on `deleteByLabels`, configured in `deployment.yml`\n\n"
                        "WARNING: This command will also delete objects which are not part of your deployment "
                        "project (anymore). It really only decides based on the `deleteByLabel` labels and does NOT "
                        "take the local target/state into account!")
@kluctl_project_args()
@image_args()
@include_exclude_args()
@misc_arguments(yes=True, dry_run=True, output_format=True)
@click.pass_obj
def delete_command_stub(obj, **kwargs):
    from kluctl.cli.commands import delete_command
    delete_command(obj, kwargs)

@cli_group.command("prune",
                   help="Searches the target cluster for prunable objects and deletes them.\n\n"
                        "Searching works by:\n\n"
                        "\b\n"
                        "  1. Search the cluster for all objects match `deleteByLabels`, as configured in `deployment.yml`\n"
                        "  2. Render the local target and list all objects.\n"
                        "  3. Remove all objects from the list of 1. that are part of the list in 2.\n")
@kluctl_project_args()
@image_args()
@include_exclude_args()
@misc_arguments(yes=True, dry_run=True, output_format=True)
@click.pass_obj
def prune_command_stub(obj, **kwargs):
    from kluctl.cli.commands import prune_command
    prune_command(obj, kwargs)

@cli_group.command("poke-images",
                   help="Replace all images in target.\n\n"
                        "This command will fully render the target and then only replace images instead of fully "
                        "deploying the target. Only images used in combination with `images.get_image(...)` are "
                        "replaced.")
@kluctl_project_args()
@image_args()
@include_exclude_args()
@misc_arguments(yes=True, dry_run=True, output_format=True, render_output_dir=True)
@click.pass_obj
def poke_images_command_stub(obj, **kwargs):
    from kluctl.cli.commands import poke_images_command
    poke_images_command(obj, kwargs)

@cli_group.command("downscale",
                   help="Downscale all deployments.\n\n"
                        "This command will downscale all Deployments, StatefulSets and CronJobs. "
                        "It is also possible to influence the behaviour with the help of annotations, as described in "
                        "the documentation.")
@kluctl_project_args()
@image_args()
@include_exclude_args()
@misc_arguments(yes=True, dry_run=True, output_format=True, render_output_dir=True)
@click.pass_obj
def downscale_command_stub(obj, **kwargs):
    from kluctl.cli.commands import downscale_command
    downscale_command(obj, kwargs)

@cli_group.command("validate",
                   help="Validates the already deployed deployment.\n\n"
                        "This means that all objects are retrieved from the cluster and checked for readiness.\n\n"
                        "TODO: This needs to be better documented!")
@kluctl_project_args()
@include_exclude_args()
@misc_arguments(output=True, render_output_dir=True)
@optgroup.option("--wait", help="Wait for the given amount of time until the deployment validates")
@optgroup.option("--sleep", help="Sleep duration between validation attempts", default="5s")
@optgroup.option("--warnings-as-errors", help="Consider warnings as failures", is_flag=True)
@click.pass_obj
def validate_command_stub(obj, **kwargs):
    from kluctl.cli.commands import validate_command
    validate_command(obj, kwargs)

@cli_group.command("render",
                   help="Renders all resources and configuration files and stores the result in either "
                        "a temporary directory or a specified directory.")
@kluctl_project_args()
@image_args()
@misc_arguments(render_output_dir=True)
@optgroup.option("--output-images",
                 help="Also output images list to given FILE. This output the same result as from the "
                      "list-images command.",
                 type=click.Path(dir_okay=False))
@optgroup.option("--output-single-yaml",
                 help="Also write all resources into a single yaml file.",
                 type=click.Path(dir_okay=False))
@click.pass_obj
def render_command_stub(obj, **kwargs):
    from kluctl.cli.commands import render_command
    render_command(obj, kwargs)

@cli_group.command("list-images",
                   help="Renders the target and outputs all images used via `images.get_image(...)`\n\n"
                        "The result is a compatible with yaml files expected by --fixed-images-file.\n\n"
                        "If fixed images (`-f/--fixed-image`) are provided, these are also taken into account, "
                        "as described in for the deploy command.")
@kluctl_project_args()
@image_args()
@include_exclude_args()
@misc_arguments(output=True)
@optgroup.option("--simple", help="Output a simplified version of the images list", is_flag=True)
@click.pass_obj
def list_images_command_stub(obj, **kwargs):
    from kluctl.cli.commands import list_images_command
    list_images_command(obj, kwargs)

@cli_group.command("list-targets",
                   help="Outputs a yaml list with all target, including dynamic targets")
@kluctl_project_args()
@misc_arguments(output=True)
@click.pass_obj
def list_targets_stub(obj, **kwargs):
    from kluctl.cli.commands import list_targets_command
    list_targets_command(obj, kwargs)

@cli_group.command("archive",
                   help="Write project and all related components into single tgz.\n\n"
                        "This archive can then be used with `--from-archive`.")
@kluctl_project_args()
@optgroup.group("Misc arguments")
@optgroup.option("--output-archive",
                 help="Path to .tgz to write project to.",
                 required=True,
                 type=click.Path(file_okay=True))
@optgroup.option("--output-metadata",
                 help="Path to .yml to write metadata to. If not specified, metadata is written into the archive.",
                 type=click.Path(file_okay=True))
@optgroup.option("--reproducible",
                 help="Make archive reproducible.",
                 is_flag=True)
@click.pass_obj
def archive_command_stub(obj, **kwargs):
    from kluctl.cli.commands import archive_command
    archive_command(obj, kwargs)
