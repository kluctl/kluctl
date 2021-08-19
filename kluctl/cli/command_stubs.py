import click
from click_option_group import optgroup

from kluctl.cli.main_cli_group import cli_group, kluctl_project_args, misc_arguments, image_args, include_exclude_args

@cli_group.command("deploy", help="Perform a deployment")
@kluctl_project_args()
@image_args()
@include_exclude_args()
@misc_arguments(yes=True, dry_run=True, parallel=True, force_apply=True, replace_on_error=True, abort_on_error=True, output_format=True)
@optgroup.option("--full-diff-after-deploy", help="Perform a full diff (no inclusion/exclusion) directly after the deploy")
@click.pass_obj
def deploy_command_stub(obj, **kwargs):
    from kluctl.cli.commands import deploy_command
    deploy_command(obj, kwargs)

@cli_group.command("diff", help="Perform a diff against the already deployed state")
@kluctl_project_args()
@image_args()
@include_exclude_args()
@misc_arguments(force_apply=True, replace_on_error=True, ignore_labels=True, ignore_order=True, output_format=True)
@click.pass_obj
def diff_command_stub(obj, **kwargs):
    from kluctl.cli.commands import diff_command
    diff_command(obj, kwargs)

@cli_group.command("delete", help="Delete the deployment, based on deleteByLabels")
@kluctl_project_args()
@image_args()
@include_exclude_args()
@misc_arguments(yes=True, dry_run=True)
@click.pass_obj
def delete_command_stub(obj, **kwargs):
    from kluctl.cli.commands import delete_command
    delete_command(obj, kwargs)

@cli_group.command("purge", help="Purges/Cleans the deployment, based on deleteByLabels")
@kluctl_project_args()
@image_args()
@include_exclude_args()
@misc_arguments(yes=True, dry_run=True)
@click.pass_obj
def purge_command_stub(obj, **kwargs):
    from kluctl.cli.commands import purge_command
    purge_command(obj, kwargs)

@cli_group.command("validate", help="Validates the deployment, based on validationRules")
@kluctl_project_args()
@include_exclude_args()
@misc_arguments(output=True)
@optgroup.option("--wait", help="Wait for the given amount of time until the deployment validates")
@optgroup.option("--sleep", help="Sleep duration between validation attempts", default="5s")
@optgroup.option("--warnings-as-errors", help="Consider warnings as failures", is_flag=True)
@click.pass_obj
def validate_command_stub(obj, **kwargs):
    from kluctl.cli.commands import validate_command
    validate_command(obj, kwargs)

@cli_group.command("render", help="Render deployment into directory")
@kluctl_project_args()
@image_args()
@optgroup.group("Misc arguments")
@optgroup.option("--output-dir", help="Write rendered output to specified directory", type=click.Path(file_okay=False))
@optgroup.option("--output-images", help="Write images list to output file", type=click.Path(dir_okay=False))
@optgroup.option("--offline", help="Go offline, meaning that kubernetes and registries are not asked for image versions", is_flag=True)
@click.pass_obj
def render_command_stub(obj, **kwargs):
    from kluctl.cli.commands import render_command
    render_command(obj, kwargs)

@cli_group.command("list-images", help="List all images queries by 'images.get_image(...)'")
@kluctl_project_args()
@image_args()
@include_exclude_args()
@misc_arguments(output=True)
@optgroup.option("--no-kubernetes", help="Don't check kubernetes for current image versions", default=False, is_flag=True)
@optgroup.option("--no-registries", help="Don't check registries for new image versions", default=False, is_flag=True)
@optgroup.option("--simple", help="Return simple list", is_flag=True)
@click.pass_obj
def list_images_command_stub(obj, **kwargs):
    from kluctl.cli.commands import list_images_command
    list_images_command(obj, kwargs)
