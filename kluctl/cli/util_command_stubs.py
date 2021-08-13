import click
from click_option_group import optgroup

from kluctl.cli.main_cli_group import cli_group, misc_arguments, project_args


@cli_group.command("list-image-tags", help="Utility command to list tags of specified images")
@misc_arguments(output=True)
@optgroup.option("--image", required=True, help="Name of the image", multiple=True)
def list_image_tags_command_stub(image, output):
    from kluctl.cli.util_commands import list_image_tags_command
    list_image_tags_command(image, output)

@cli_group.command("helm-pull", help="Recursively search for 'helm-chart.yml' files and pull specified Helm Charts")
@project_args(with_a=False)
def helm_pull_command_stub(**kwargs):
    from kluctl.cli.util_commands import helm_pull_command
    helm_pull_command(kwargs)

@cli_group.command("helm-update", help="Recursively search for 'helm-chart.yml' files and check for new versions")
@project_args(with_a=False)
@optgroup.group("Misc arguments")
@optgroup.option("--upgrade", help="Write new versions into helm-chart.yml and perform helm-pull afterwards", is_flag=True)
@optgroup.option("--commit", help="Create a git commit for every updated chart", is_flag=True)
def helm_update_command_stub(upgrade, commit, **kwargs):
    from kluctl.cli.util_commands import helm_update_command
    helm_update_command(upgrade, commit, kwargs)

@cli_group.command("check-image-updates",
                   help="Render deployment and check if any images have new tags available. "
                        "This is based on a best effort approach and might give many false-positives.")
@project_args()
@click.pass_obj
def check_image_updates_stub(obj, **kwargs):
    from kluctl.cli.util_commands import check_image_updates
    check_image_updates(obj, kwargs)