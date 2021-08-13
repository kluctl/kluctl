import click
from click_option_group import optgroup

from kluctl.cli.main_cli_group import project_args, cli_group


@cli_group.command("seal", help="Seal secrets based on sealme-conf.yml")
@project_args()
@optgroup.group("Misc arguments")
@optgroup.option("--secrets-dir", help="Directory of unencrypted secret files (default is $PWD)", default='.', type=click.Path(exists=True, file_okay=False))
@optgroup.option("--force-reseal", help="Force re-sealing even if a secret is determined as unchanged.", is_flag=True)
@click.pass_obj
def seal_command_stub(obj, **kwargs):
    from kluctl.seal.seal_command import seal_command
    seal_command(obj, kwargs)
