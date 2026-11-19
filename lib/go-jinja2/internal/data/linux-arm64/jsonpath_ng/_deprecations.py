import sys
import types
import warnings

_DEPRECATED_CLASS_NAMES = frozenset({"Union", "Intersect"})


def _deprecated_class_message(name: str) -> str:
    return (
        f"`{name}` is deprecated. "
        "It will be removed in version 2.0.0, along with the operator that creates it."
    )


def _warn_about_deprecated_syntax(message: str) -> None:
    """Warn about syntax that is deprecated, from inside a parser production.

    The parser's productions run deep inside of PLY,
    so a fixed `stacklevel` would attribute the warning to jsonpath-ng's internals.
    The warning is instead attributed to the first caller outside jsonpath-ng,
    which is the code that called `parse()`.
    """

    stacklevel = 1
    frame: types.FrameType | None = sys._getframe(0)
    while frame is not None:
        module = frame.f_globals.get("__name__", "")
        if module != "jsonpath_ng" and not module.startswith("jsonpath_ng."):
            break
        stacklevel += 1
        frame = frame.f_back

    warnings.warn(message, DeprecationWarning, stacklevel=stacklevel)
