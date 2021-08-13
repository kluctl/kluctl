import click

from kluctl.cli.recursive_click_context import RecursiveClickContext

click.BaseCommand.context_class = RecursiveClickContext

from kluctl.cli import main_cli_group
from kluctl.cli import util_command_stubs
from kluctl.cli import command_stubs
from kluctl.cli import seal_command_stubs
