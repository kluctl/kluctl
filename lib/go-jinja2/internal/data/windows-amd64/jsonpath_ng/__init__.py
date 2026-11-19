import warnings

from ._deprecations import _DEPRECATED_CLASS_NAMES
from ._deprecations import _deprecated_class_message
from .jsonpath import *  # noqa
from .parser import parse  # noqa


def __getattr__(name: str) -> object:
    if name in _DEPRECATED_CLASS_NAMES:
        from . import jsonpath

        warnings.warn(_deprecated_class_message(name), DeprecationWarning, stacklevel=2)
        return getattr(jsonpath, f"_{name}")
    if name == "__version__":
        msg = (
            "The `__version__` attribute is deprecated. "
            "It will be removed in version 2.0.0. "
            'Use `importlib.metadata.version("jsonpath-ng")` instead.'
        )
        warnings.warn(msg, DeprecationWarning, stacklevel=2)
        import importlib.metadata

        return importlib.metadata.version("jsonpath-ng")
    raise AttributeError(f"module '{__name__}' has no attribute '{name}'")
