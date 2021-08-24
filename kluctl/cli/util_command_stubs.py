import click
from click_option_group import optgroup

from kluctl.cli.main_cli_group import cli_group, misc_arguments, kluctl_project_args


@cli_group.command("list-image-tags",
                   help="Queries the tags for the given images.")
@misc_arguments(output=True)
@optgroup.option("--image", required=True, help="Name of the image. Can be specified multiple times", multiple=True)
def list_image_tags_command_stub(image, output):
    from kluctl.cli.util_commands import list_image_tags_command
    list_image_tags_command(image, output)

@cli_group.command("helm-pull",
                   help="Recursively searches for `helm-chart.yml` files and pulls the specified Helm charts.\n\n"
                        "The Helm charts are stored under the sub-directory `charts/<chart-name>` next to the "
                        "`helm-chart.yml`. These Helm charts are meant to be added to version control so that "
                        "pulling is only needed when really required (e.g. when the chart version changes).")
@optgroup.group("Project arguments")
@optgroup.option("--local-deployment",
                 help="Local deployment directory. Defaults to current directory",
                 default=".",
                 type=click.Path(file_okay=False))
def helm_pull_command_stub(**kwargs):
    from kluctl.cli.util_commands import helm_pull_command
    helm_pull_command(kwargs)

@cli_group.command("helm-update",
                   help="Recursively searches for `helm-chart.yml` files and checks for new available versions.\n\n"
                        "Optionally performs the actual upgrade and/or add a commit to version control.")
@optgroup.group("Project arguments")
@optgroup.option("--local-deployment",
                 help="Local deployment directory. Defaults to current directory",
                 default=".",
                 type=click.Path(file_okay=False))
@optgroup.group("Misc arguments")
@optgroup.option("--upgrade", help="Write new versions into helm-chart.yml and perform helm-pull afterwards", is_flag=True)
@optgroup.option("--commit", help="Create a git commit for every updated chart", is_flag=True)
def helm_update_command_stub(upgrade, commit, **kwargs):
    from kluctl.cli.util_commands import helm_update_command
    helm_update_command(upgrade, commit, kwargs)

@cli_group.command("check-image-updates",
                   help="Render deployment and check if any images have new tags available.\n\n"
                        "This is based on a best effort approach and might give many false-positives.")
@kluctl_project_args()
@click.pass_obj
def check_image_updates_stub(obj, **kwargs):
    from kluctl.cli.util_commands import check_image_updates
    check_image_updates(obj, kwargs)