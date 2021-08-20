import click
from click_option_group import optgroup

from kluctl.cli.main_cli_group import kluctl_project_args, cli_group


@cli_group.command("seal",
                   help="Seal secrets based on target's sealingConfig.\n\n"
                        "Loads all secrets from the specified secrets sets from the target's sealingConfig and "
                        "then renders the target, including all files with the `.sealme` extension. Then runs "
                        "kubeseal on each `.sealme` file and stores secrets in the directory specified by "
                        "`--local-sealed-secrets`, using the outputPattern from your deployment project.\n\n"
                        "If no `--target` is specified, sealing is performed for all targets.")
@kluctl_project_args()
@optgroup.group("Misc arguments")
@optgroup.option("--secrets-dir",
                 help="Specifies where to find unencrypted secret files. The given directory is NOT meant to be part "
                      "of your source repository! The given path only matters for secrets of type 'path'. Defaults "
                      "to the current working directory.",
                 default='.', type=click.Path(exists=True, file_okay=False))
@optgroup.option("--force-reseal",
                 help="Lets kluctl ignore secret hashes found in already sealed secrets and thus forces "
                      "resealing of those.",
                 is_flag=True)
@click.pass_obj
def seal_command_stub(obj, **kwargs):
    from kluctl.seal.seal_command import seal_command
    seal_command(obj, kwargs)
