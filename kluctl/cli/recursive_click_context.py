import click


class RecursiveClickContext(click.Context):
    def __init__(self, command, parent=None, default_map=None, auto_envvar_prefix=None, *args, **kwargs):
        default_map = default_map or (parent.default_map if parent else None)
        auto_envvar_prefix = auto_envvar_prefix or (parent.auto_envvar_prefix if parent else None)
        super().__init__(command, parent=parent, default_map=default_map, auto_envvar_prefix=auto_envvar_prefix, *args, **kwargs)

class RecursiveContextGroup(click.Group):
    context_class = RecursiveClickContext
