from __future__ import annotations

import logging
import re
import typing
import warnings

from jsonpath_ng._deprecations import _DEPRECATED_CLASS_NAMES
from jsonpath_ng._deprecations import _deprecated_class_message
from jsonpath_ng.exceptions import JSONPathError

# Get logger name
logger = logging.getLogger(__name__)

# Turn on/off the automatic creation of id attributes
# ... could be a kwarg pervasively but uses are rare and simple today
auto_id_field: str | None = None

NOT_SET = object()
LIST_KEY = object()


Container: typing.TypeAlias = dict[typing.Any, typing.Any] | list[typing.Any]
FilterFunction: typing.TypeAlias = typing.Callable[[Container], bool]


class JSONPath:
    """
    The base class for JSONPath abstract syntax; those
    methods stubbed here are the interface to supported
    JSONPath semantics.
    """

    def find(self, data: typing.Any) -> list[DatumInContext]:
        """
        All `JSONPath` types support `find()`,
        which returns an iterable of `DatumInContext`s.
        They keep track of the path followed to the current location,
        so if the calling code has some opinion about that,
        it can be passed in here as a starting point.
        """
        raise NotImplementedError()

    def find_or_create(self, data: typing.Any) -> list[DatumInContext]:
        return self.find(data)

    def update(self, data: typing.Any, val: typing.Any) -> typing.Any:
        """
        Returns `data` with the specified path replaced by `val`. Only updates
        if the specified path exists.
        """

        raise NotImplementedError()

    def update_or_create(self, data: typing.Any, val: typing.Any) -> typing.Any:
        return self.update(data, val)

    def _update_with_root(
        self, data: typing.Any, val: typing.Any, root: typing.Any
    ) -> typing.Any:
        """
        Like `update()`, but also told the root of the query argument.

        Only paths that evaluate `$` (filters) need it, so the default ignores it.
        """

        return self.update(data, val)

    def _filter_with_root(
        self, fn: FilterFunction, data: Container, root: typing.Any
    ) -> Container | None:
        """Like `filter()`, but also told the root of the query argument."""

        return self.filter(fn, data)

    def filter(self, fn: FilterFunction, data: Container) -> Container | None:
        """
        Returns `data` with the specified path filtering nodes according
        the filter evaluation result returned by the filter function.

        Arguments:
            fn (function): unary function that accepts one argument
                and returns bool.
            data (dict|list|tuple): JSON object to filter.
        """

        raise NotImplementedError()

    def child(self, child: JSONPath) -> JSONPath:
        """
        Equivalent to Child(self, next) but with some canonicalization
        """
        if isinstance(self, This) or isinstance(self, Root):
            return child
        elif isinstance(child, This):
            return self
        elif isinstance(child, Root):
            return child
        else:
            return Child(self, child)

    def make_datum(self, value: DatumInContext | typing.Any) -> DatumInContext:
        if isinstance(value, DatumInContext):
            return value
        else:
            return DatumInContext(value, path=Root(), context=None)


class DatumInContext:
    """
    Represents a datum along a path from a context.

    Essentially a zipper but with a structure represented by JsonPath,
    and where the context is more of a parent pointer than a proper
    representation of the context.

    For quick-and-dirty work, this proxies any non-special attributes
    to the underlying datum, but the actual datum can (and usually should)
    be retrieved via the `value` attribute.

    To place `datum` within another, use `datum.in_context(context=..., path=...)`
    which extends the path. If the datum already has a context, it places the entire
    context within that passed in, so an object can be built from the inside
    out.
    """

    @classmethod
    def wrap(cls, data: typing.Any) -> DatumInContext:
        if isinstance(data, cls):
            return data
        else:
            return cls(data)

    def __init__(
        self,
        value: typing.Any,
        path: JSONPath | None = None,
        context: DatumInContext | typing.Any | None = None,
    ) -> None:
        self.__value__ = value
        self.path = path or This()
        # This is a hot path, so avoid the overhead of calling `wrap()`.
        if context is not None and not isinstance(context, DatumInContext):
            context = DatumInContext(context)
        self.context: DatumInContext | None = context

    @property
    def value(self) -> typing.Any:
        return self.__value__

    @value.setter
    def value(self, value: typing.Any) -> None:
        if self.context is not None and self.context.value is not None:
            self.path.update(self.context.value, value)
        self.__value__ = value

    def in_context(self, context: typing.Any, path: JSONPath) -> DatumInContext:
        context = DatumInContext.wrap(context)

        if self.context:
            return DatumInContext(
                value=self.value,
                path=self.path,
                context=context.in_context(path=path, context=context),
            )
        else:
            return DatumInContext(value=self.value, path=path, context=context)

    @property
    def full_path(self) -> JSONPath:
        return (
            self.path
            if self.context is None
            else self.context.full_path.child(self.path)
        )

    @property
    def id_pseudopath(self) -> JSONPath:
        """
        Looks like a path, but with ids stuck in when available
        """
        try:
            pseudopath: JSONPath = Fields._compliant_constructor(
                str(self.value[auto_id_field])
            )
        except (
            TypeError,
            AttributeError,
            KeyError,
        ):  # This may not be all the interesting exceptions
            pseudopath = self.path

        if self.context:
            return self.context.id_pseudopath.child(pseudopath)
        else:
            return pseudopath

    def __repr__(self) -> str:
        return "{}(value={!r}, path={!r}, context={!r})".format(
            self.__class__.__name__, self.value, self.path, self.context
        )

    def __eq__(self, other: typing.Any) -> bool:
        return (
            isinstance(other, DatumInContext)
            and other.value == self.value
            and other.path == self.path
            and self.context == other.context
        )


class AutoIdForDatum(DatumInContext):
    """
    This behaves like a DatumInContext, but the value is
    always the path leading up to it, not including the "id",
    and with any "id" fields along the way replacing the prior
    segment of the path

    For example, it will make "foo.bar.id" return a datum
    that behaves like DatumInContext(value="foo.bar", path="foo.bar.id").

    This is disabled by default; it can be turned on by
    settings the `auto_id_field` global to a value other
    than `None`.
    """

    def __init__(self, datum: DatumInContext, id_field: str | None = None) -> None:
        """
        Invariant is that datum.path is the path from context to datum. The auto id
        will either be the id in the datum (if present) or the id of the context
        followed by the path to the datum.

        The path to this datum is always the path to the context, the path to the
        datum, and then the auto id field.
        """
        self.datum = datum
        self.id_field = id_field or auto_id_field

    @property
    def value(self) -> str:
        return str(self.datum.id_pseudopath)

    @value.setter
    def value(self, value: typing.Any) -> typing.Never:
        raise JSONPathError("The value of an auto ID cannot be set.")

    @property
    def path(self) -> str | None:  # type: ignore[override]
        return self.id_field

    @property
    def context(self) -> DatumInContext:  # type: ignore[override]
        return self.datum

    def __repr__(self) -> str:
        return f"{self.__class__.__name__}({self.datum!r})"

    def in_context(self, context: typing.Any, path: JSONPath) -> AutoIdForDatum:
        return AutoIdForDatum(self.datum.in_context(context=context, path=path))

    def __eq__(self, other: typing.Any) -> bool:
        return (
            isinstance(other, AutoIdForDatum)
            and other.datum == self.datum
            and self.id_field == other.id_field
        )


class Root(JSONPath):
    """
    The JSONPath referring to the "root" object. Concrete syntax is '$'.
    The root is the topmost datum without any context attached.
    """

    def find(self, data: DatumInContext | typing.Any) -> list[DatumInContext]:
        if not isinstance(data, DatumInContext):
            return [DatumInContext(data, path=Root(), context=None)]
        else:
            if data.context is None:
                return [DatumInContext(data.value, context=None, path=Root())]
            else:
                return Root().find(data.context)

    def update(self, data: typing.Any, val: typing.Any) -> typing.Any:
        return val

    def filter(self, fn: FilterFunction, data: Container) -> Container | None:
        return data if fn(data) else None

    def __str__(self) -> str:
        return "$"

    def __repr__(self) -> str:
        return "Root()"

    def __eq__(self, other: typing.Any) -> bool:
        return isinstance(other, Root)

    def __hash__(self) -> int:
        return hash("$")


class This(JSONPath):
    """
    The JSONPath referring to the current datum. Concrete syntax is '@'.
    """

    def find(self, datum: typing.Any) -> list[DatumInContext]:
        return [DatumInContext.wrap(datum)]

    def update(self, data: typing.Any, val: typing.Any) -> typing.Any:
        return val

    def filter(self, fn: FilterFunction, data: Container) -> Container | None:
        return data if fn(data) else None

    def __str__(self) -> str:
        return "`this`"

    def __repr__(self) -> str:
        return "This()"

    def __eq__(self, other: typing.Any) -> bool:
        return isinstance(other, This)

    def __hash__(self) -> int:
        return hash("this")


class Child(JSONPath):
    """
    JSONPath that first matches the left, then the right.
    Concrete syntax is <left> '.' <right>
    """

    def __init__(self, left: JSONPath, right: JSONPath) -> None:
        self.left = left
        self.right = right

    def find(self, datum: typing.Any) -> list[DatumInContext]:
        """
        Extra special case: auto ids do not have children,
        so cut it off right now rather than auto id the auto id
        """

        return [
            submatch
            for subdata in self.left.find(datum)
            if not isinstance(subdata, AutoIdForDatum)
            for submatch in self.right.find(subdata)
        ]

    def update(self, data: typing.Any, val: typing.Any) -> typing.Any:
        return self._update_with_root(data, val, data)

    def _update_with_root(
        self, data: typing.Any, val: typing.Any, root: typing.Any
    ) -> typing.Any:
        for datum in self.left.find(data):
            self.right._update_with_root(datum.value, val, root)
        return data

    def find_or_create(self, datum: typing.Any) -> list[DatumInContext]:
        datum = DatumInContext.wrap(datum)
        submatches = []
        for subdata in self.left.find_or_create(datum):
            if isinstance(subdata, AutoIdForDatum):
                # Extra special case: auto ids do not have children,
                # so cut it off right now rather than auto id the auto id
                continue
            for submatch in self.right.find_or_create(subdata):
                submatches.append(submatch)
        return submatches

    def update_or_create(self, data: typing.Any, val: typing.Any) -> typing.Any:
        for datum in self.left.find_or_create(data):
            self.right.update_or_create(datum.value, val)
        return _clean_list_keys(data)

    def filter(self, fn: FilterFunction, data: Container) -> Container | None:
        return self._filter_with_root(fn, data, data)

    def _filter_with_root(
        self, fn: FilterFunction, data: Container, root: typing.Any
    ) -> Container | None:
        for datum in self.left.find(data):
            self.right._filter_with_root(fn, datum.value, root)
        return data

    def __eq__(self, other: typing.Any) -> bool:
        return (
            isinstance(other, Child)
            and self.left == other.left
            and self.right == other.right
        )

    def __str__(self) -> str:
        # Special case: If the right side is a `SortedThis` or `Filter` instance,
        # do not inject a period between the left and right sides.
        # Adding a period would corrupt the syntax and prevent re-parsing.
        # Current module design creates circular imports, so imports happen here.
        from .ext.filter import Filter
        from .ext.iterable import SortedThis

        if isinstance(self.right, (SortedThis, Filter)):
            return f"{self.left}{self.right}"

        return f"{self.left}.{self.right}"

    def __repr__(self) -> str:
        return f"{self.__class__.__name__}({self.left!r}, {self.right!r})"

    def __hash__(self) -> int:
        return hash((self.left, self.right))


class Parent(JSONPath):
    """
    JSONPath that matches the parent node of the current match.
    Will crash if no such parent exists.
    Available via named operator `parent`.
    """

    def find(self, datum: DatumInContext) -> list[DatumInContext]:
        datum = DatumInContext.wrap(datum)
        context = datum.context
        assert context is not None
        return [context]

    def __eq__(self, other: typing.Any) -> bool:
        return isinstance(other, Parent)

    def __str__(self) -> str:
        return "`parent`"

    def __repr__(self) -> str:
        return "Parent()"

    def __hash__(self) -> int:
        return hash("parent")


class Where(JSONPath):
    """
    JSONPath that first matches the left, and then
    filters for only those nodes that have
    a match on the right.

    WARNING: Subject to change. May want to have "contains"
    or some other better word for it.
    """

    def __init__(self, left: JSONPath, right: JSONPath) -> None:
        self.left = left
        self.right = right

    def find(self, data: typing.Any) -> list[DatumInContext]:
        return [subdata for subdata in self.left.find(data) if self.right.find(subdata)]

    def update(self, data: typing.Any, val: typing.Any) -> typing.Any:
        for datum in self.find(data):
            datum.path.update(data, val)
        return data

    def filter(self, fn: FilterFunction, data: Container) -> Container | None:
        for datum in self.find(data):
            datum.path.filter(fn, datum.value)
        return data

    def __str__(self) -> str:
        return f"{self.left} where {self.right}"

    def __eq__(self, other: typing.Any) -> bool:
        return (
            isinstance(other, Where)
            and other.left == self.left
            and other.right == self.right
        )

    def __hash__(self) -> int:
        return hash((self.left, self.right))


class WhereNot(Where):
    """
    Identical to ``Where``, but filters for only those nodes that
    do *not* have a match on the right.

    >>> jsonpath = WhereNot(Fields('spam'), Fields('spam'))
    >>> jsonpath.find({"spam": {"spam": 1}})
    []
    >>> matches = jsonpath.find({"spam": 1})
    >>> matches[0].value
    1

    """

    def find(self, data: typing.Any) -> list[DatumInContext]:
        return [
            subdata for subdata in self.left.find(data) if not self.right.find(subdata)
        ]

    def __str__(self) -> str:
        return f"{self.left} wherenot {self.right}"

    def __eq__(self, other: typing.Any) -> bool:
        return (
            isinstance(other, WhereNot)
            and other.left == self.left
            and other.right == self.right
        )

    def __hash__(self) -> int:
        return hash((self.left, self.right))


class Descendants(JSONPath):
    """
    JSONPath that matches first the left expression then any descendant
    of it which matches the right expression.
    """

    def __init__(self, left: JSONPath, right: JSONPath) -> None:
        self.left = left
        self.right = right

    def find(self, datum: typing.Any) -> list[DatumInContext]:
        # <left> .. <right> ==> <left> . (<right> | *..<right> | [*]..<right>)
        #
        # This walks the data directly instead of delegating to that equivalence.

        # Get all left matches into a list.
        # Some `find()` implementations (e.g. `Filter`, `SortedThis`)
        # return a bare datum instead of a list, so this is typed as `Any`.
        left_matches: typing.Any = self.left.find(datum)

        # TODO: Remove this condition once all `find()` methods return lists.
        if not isinstance(left_matches, list):
            left_matches = [left_matches]

        # Walk the data depth-first and in pre-order using an explicit stack.
        # The next datum to visit is at the end of the list (which is cheap to pop),
        # so children are pushed in reverse to be visited in their natural order.
        right_find = self.right.find
        matches: list[DatumInContext] = []
        stack = left_matches[::-1]
        while stack:
            datum = stack.pop()
            matches.extend(right_find(datum))

            value = datum.value
            if isinstance(value, list):
                children = [
                    DatumInContext(child, context=datum, path=Index(i))
                    for i, child in enumerate(value)
                ]
                children.reverse()
                stack.extend(children)
            elif isinstance(value, dict):
                stack.extend(
                    DatumInContext(
                        child, context=datum, path=Fields._compliant_constructor(field)
                    )
                    for field, child in reversed(value.items())
                )

        return matches

    def is_singular(self) -> typing.Literal[False]:
        return False

    def update(self, data: typing.Any, val: typing.Any) -> typing.Any:
        return self._update_with_root(data, val, data)

    def _update_with_root(
        self, data: typing.Any, val: typing.Any, root: typing.Any
    ) -> typing.Any:
        # Get all left matches into a list.
        # Some `find()` implementations (e.g. `Filter`, `SortedThis`)
        # return a bare datum instead of a list, so this is typed as `Any`.
        left_matches: typing.Any = self.left.find(data)

        # TODO: Remove this condition once all `find()` methods return lists.
        if not isinstance(left_matches, list):
            left_matches = [left_matches]

        # Walk the data depth-first and in pre-order using an explicit stack.
        # Children are pushed in reverse to be visited in their natural order.
        stack = [submatch.value for submatch in reversed(left_matches)]
        while stack:
            node = stack.pop()

            # Update only mutable values corresponding to JSON types.
            if not isinstance(node, (list, dict)):
                continue

            self.right._update_with_root(node, val, root)

            # Manually visit every child and recurse just the right-hand pattern.
            # The children are read after the update, which may have changed them.
            stack.extend(reversed(node if isinstance(node, list) else node.values()))

        return data

    def filter(self, fn: FilterFunction, data: Container) -> Container | None:
        return self._filter_with_root(fn, data, data)

    def _filter_with_root(
        self, fn: FilterFunction, data: Container, root: typing.Any
    ) -> Container | None:
        # Get all left matches into a list.
        # Some `find()` implementations (e.g. `Filter`, `SortedThis`)
        # return a bare datum instead of a list, so this is typed as `Any`.
        left_matches: typing.Any = self.left.find(data)

        # TODO: Remove this condition once all `find()` methods return lists.
        if not isinstance(left_matches, list):
            left_matches = [left_matches]

        stack = [submatch.value for submatch in reversed(left_matches)]
        while stack:
            node = stack.pop()

            # Filter only mutable values corresponding to JSON types.
            if not isinstance(node, (list, dict)):
                continue

            self.right._filter_with_root(fn, node, root)

            # The children are read after the filter, which may have removed some.
            stack.extend(reversed(node if isinstance(node, list) else node.values()))

        return data

    def __str__(self) -> str:
        return f"({self.left}..{self.right})"

    def __eq__(self, other: typing.Any) -> bool:
        return (
            isinstance(other, Descendants)
            and self.left == other.left
            and self.right == other.right
        )

    def __repr__(self) -> str:
        return f"{self.__class__.__name__}({self.left!r}, {self.right!r})"

    def __hash__(self) -> int:
        return hash((self.left, self.right))


class _Union(JSONPath):
    """
    JSONPath that returns the union of the results of each match.
    This is pretty shoddily implemented for now. The nicest semantics
    in case of mismatched bits (list vs atomic) is to put
    them all in a list, but I haven't done that yet.

    WARNING: Any appearance of this being the _concatenation_ is
    coincidence. It may even be a bug! (or laziness)
    """

    def __init__(self, left: JSONPath, right: JSONPath) -> None:
        self.left = left
        self.right = right

    def is_singular(self) -> typing.Literal[False]:
        return False

    def find(self, data: typing.Any) -> list[DatumInContext]:
        return self.left.find(data) + self.right.find(data)

    def __eq__(self, other: typing.Any) -> bool:
        return (
            isinstance(other, _Union)
            and self.left == other.left
            and self.right == other.right
        )

    def __hash__(self) -> int:
        return hash((self.left, self.right))

    def __repr__(self) -> str:
        return f"Union({self.left} | {self.right})"

    def __str__(self) -> str:
        return f"{self.left} | {self.right}"


class _Intersect(JSONPath):
    """
    JSONPath for bits that match *both* patterns.

    This can be accomplished a couple of ways. The most
    efficient is to actually build the intersected
    AST as in building a state machine for matching the
    intersection of regular languages. The next
    idea is to build a filtered data and match against
    that.
    """

    def __init__(self, left: JSONPath, right: JSONPath) -> None:
        self.left = left
        self.right = right

    def is_singular(self) -> typing.Literal[False]:
        return False

    def find(self, data: typing.Any) -> list[DatumInContext]:
        raise NotImplementedError()

    def __eq__(self, other: typing.Any) -> bool:
        return (
            isinstance(other, _Intersect)
            and self.left == other.left
            and self.right == other.right
        )

    def __hash__(self) -> int:
        return hash((self.left, self.right))

    def __repr__(self) -> str:
        return f"Intersect({self.left} & {self.right})"

    def __str__(self) -> str:
        return f"{self.left} & {self.right}"


class Fields(JSONPath):
    """
    JSONPath referring to some field of the current object.
    Concrete syntax is comma-separated field names.

    DEPRECATED: If '*' is any of the field names given to the constructor,
    then every field will be returned, and so will every element of an array.
    This will change in version 2.0.0, when '*' will be an ordinary field name.
    Use `Wildcard()` to select every child.
    """

    # True only for a hand-built `Fields('*')`, which is a deprecated wildcard.
    # A name that came from a query or from the data is never a wildcard.
    _deprecated_wildcard_use: bool

    def __init__(self, *fields: str) -> None:
        self._deprecated_wildcard_use = "*" in fields
        if self._deprecated_wildcard_use:
            msg = (
                "Using `Fields('*')` as a wildcard is deprecated. "
                "In version 2.0.0, `Fields('*')` will select only a field named `*`. "
                "Use `Wildcard()` to select every child."
            )
            warnings.warn(msg, DeprecationWarning, stacklevel=2)
        self.fields = fields

    @classmethod
    def _compliant_constructor(cls, *fields: str) -> Fields:
        """
        Create a node whose names are all plain names.

        This method solely exists to support instantiation
        without triggering a wildcard-related DeprecationWarning.
        This method will be removed in version 2.0.0.
        """

        node = cls.__new__(cls)
        node.fields = fields
        node._deprecated_wildcard_use = False
        return node

    def _is_wildcard_on_array(self, value: typing.Any) -> bool:
        return self._deprecated_wildcard_use and isinstance(value, (list, tuple))

    @staticmethod
    def get_field_datum(
        datum: DatumInContext, field: str, create: bool
    ) -> DatumInContext | None:
        if field == auto_id_field:
            return AutoIdForDatum(datum)
        try:
            field_value = datum.value.get(field, NOT_SET)
            if field_value is NOT_SET:
                if create:
                    datum.value[field] = field_value = {}
                else:
                    return None
            return DatumInContext(
                field_value, path=Fields._compliant_constructor(field), context=datum
            )
        except (TypeError, AttributeError):
            return None

    def reified_fields(self, datum: DatumInContext) -> tuple[str, ...]:
        if not self._deprecated_wildcard_use:
            return self.fields
        else:
            try:
                fields = tuple(datum.value.keys())
                return fields if auto_id_field is None else fields + (auto_id_field,)
            except AttributeError:
                return ()

    def find(self, datum: typing.Any) -> list[DatumInContext]:
        return self._find_base(datum, create=False)

    def find_or_create(self, datum: typing.Any) -> list[DatumInContext]:
        return self._find_base(datum, create=True)

    def _find_base(self, datum: typing.Any, create: bool) -> list[DatumInContext]:
        datum = DatumInContext.wrap(datum)
        if self._is_wildcard_on_array(datum.value):
            return _find_array_elements(datum)
        field_data = [
            self.get_field_datum(datum, field, create)
            for field in self.reified_fields(datum)
        ]
        return [fd for fd in field_data if fd is not None]

    def update(self, data: typing.Any, val: typing.Any) -> typing.Any:
        if self._deprecated_wildcard_use and isinstance(data, list):
            return _update_array_elements(data, val)
        if not hasattr(data, "get"):
            return data
        return self._update_base(data, val, create=False)

    def update_or_create(self, data: typing.Any, val: typing.Any) -> typing.Any:
        return self._update_base(data, val, create=True)

    def _update_base(
        self, data: typing.Any, val: typing.Any, create: bool
    ) -> typing.Any:
        if data is not None:
            for field in self.reified_fields(DatumInContext.wrap(data)):
                if create and field not in data:
                    data[field] = {}
                if type(data) is not bool and field in data:
                    if callable(val):
                        val_result = val(data[field], data, field)
                        if val_result is not None:
                            data[field] = val_result
                    else:
                        data[field] = val
        return data

    def filter(self, fn: FilterFunction, data: Container) -> Container:
        if self._deprecated_wildcard_use and isinstance(data, list):
            return _filter_array_elements(fn, data)
        if data is not None and isinstance(data, dict):
            for field in self.reified_fields(DatumInContext.wrap(data)):
                if field in data:
                    if fn(data[field]):
                        data.pop(field)
        return data

    def __str__(self) -> str:
        # Enclose fields in quotes as needed.
        # This is a conservative check, and is biased toward quoting fields.
        # An `@` may begin a name only if a letter or underscore follows it.
        # Otherwise, the extended parser reads the `@` as the current node.
        rendered_fields: list[str] = []
        for field in self.fields:
            if (field == "*" and self._deprecated_wildcard_use) or re.match(
                r"^@?[A-Za-z_][A-Za-z0-9_@-]*$", field
            ):
                rendered_fields.append(field)
            else:
                rendered_fields.append(f"{field!r}")
        return ",".join(rendered_fields)

    def __repr__(self) -> str:
        fields = ",".join(map(repr, self.fields))
        if not self._deprecated_wildcard_use and "*" in self.fields:
            # Distinguish a field named `*` from the deprecated wildcard.
            return f"{self.__class__.__name__}._compliant_constructor({fields})"
        return f"{self.__class__.__name__}({fields})"

    def __eq__(self, other: typing.Any) -> bool:
        return (
            isinstance(other, Fields)
            and tuple(self.fields) == tuple(other.fields)
            and self._deprecated_wildcard_use == other._deprecated_wildcard_use
        )

    def __hash__(self) -> int:
        return hash((tuple(self.fields), self._deprecated_wildcard_use))


class Wildcard(JSONPath):
    """
    JSONPath that matches every child of an object or array.
    Concrete syntax is `*`, as in `$.*` and `$[*]`.

    RFC 9535, section 2.3.2: a wildcard selects every member of an object
    and every element of an array, and selects nothing from a scalar.
    """

    @staticmethod
    def _keys(value: typing.Any) -> tuple[typing.Any, ...]:
        try:
            return tuple(value.keys())
        except AttributeError:
            return ()

    def find(self, datum: typing.Any) -> list[DatumInContext]:
        datum = DatumInContext.wrap(datum)
        if isinstance(datum.value, (list, tuple)):
            return _find_array_elements(datum)
        if isinstance(datum.value, dict) and auto_id_field is None:
            # Without auto ids, every member exists, so skip the per-name lookups.
            make_path = Fields._compliant_constructor
            return [
                DatumInContext(value, path=make_path(key), context=datum)
                for key, value in datum.value.items()
            ]
        try:
            keys = tuple(datum.value.keys())
        except AttributeError:
            return []
        if auto_id_field is not None:
            keys += (auto_id_field,)
        field_data = [Fields.get_field_datum(datum, key, False) for key in keys]
        return [fd for fd in field_data if fd is not None]

    def update(self, data: typing.Any, val: typing.Any) -> typing.Any:
        if isinstance(data, list):
            return _update_array_elements(data, val)
        if not hasattr(data, "get"):
            return data
        return self.update_or_create(data, val)

    def update_or_create(self, data: typing.Any, val: typing.Any) -> typing.Any:
        # Children that already exist are the only ones that can be updated,
        # so there is nothing to create.
        return Fields._compliant_constructor(*self._keys(data))._update_base(
            data, val, create=False
        )

    def filter(self, fn: FilterFunction, data: Container) -> Container:
        if isinstance(data, list):
            return _filter_array_elements(fn, data)
        if isinstance(data, dict):
            for key, value in list(data.items()):
                if fn(value):
                    del data[key]
        return data

    def __str__(self) -> str:
        return "*"

    def __repr__(self) -> str:
        return f"{self.__class__.__name__}()"

    def __eq__(self, other: typing.Any) -> bool:
        return isinstance(other, Wildcard)

    def __hash__(self) -> int:
        return hash(Wildcard)


class Index(JSONPath):
    """
    JSONPath that matches indices of the current datum, or none if not large enough.
    Concrete syntax is brackets.

    WARNING: If the datum is None or not long enough,
    it will not crash but will not match anything.
    NOTE: For the concrete syntax of `[*]`, the abstract syntax is `Wildcard()`.
    """

    def __init__(self, *indices: int) -> None:
        self.indices = indices

    def find(self, datum: typing.Any) -> list[DatumInContext]:
        return self._find_base(datum, create=False)

    def find_or_create(self, datum: typing.Any) -> list[DatumInContext]:
        return self._find_base(datum, create=True)

    def _find_base(self, datum: typing.Any, create: bool) -> list[DatumInContext]:
        datum = DatumInContext.wrap(datum)
        if create:
            if datum.value == {}:
                datum.value = _create_list_key(datum.value)
            self._pad_value(datum.value)
        rv: list[DatumInContext] = []
        if not isinstance(datum.value, (list, tuple)):
            # Integer indices only apply to arrays.
            return rv
        for index in self.indices:
            # invalid indices do not crash, return [] instead
            if datum.value and -len(datum.value) <= index < len(datum.value):
                rv += [
                    DatumInContext(datum.value[index], path=Index(index), context=datum)
                ]
        return rv

    def update(self, data: typing.Any, val: typing.Any) -> typing.Any:
        if not isinstance(data, list):
            return data
        return self._update_base(data, val, create=False)

    def update_or_create(self, data: typing.Any, val: typing.Any) -> typing.Any:
        return self._update_base(data, val, create=True)

    def _update_base(
        self, data: typing.Any, val: typing.Any, create: bool
    ) -> typing.Any:
        if create:
            if data == {}:
                data = _create_list_key(data)
            self._pad_value(data)
        if callable(val):
            for index in self.indices:
                if not -len(data) <= index < len(data):
                    continue
                val_result = val(data[index], data, index)
                if val_result is not None:
                    data[index] = val_result
        else:
            for index in self.indices:
                if -len(data) <= index < len(data):
                    try:
                        if isinstance(val, list):
                            # allows somelist[5,1,2] = [
                            #   some_value, another_value, third_value
                            # ]
                            data[index] = val.pop(0)
                        else:
                            data[index] = val
                    except Exception as e:
                        raise e
        return data

    def filter(self, fn: FilterFunction, data: Container) -> Container:
        if not isinstance(data, list):
            return data

        # Resolve the indices to distinct, valid positions before removing anything.
        # Although the indices are normalized and deduplicated,
        # their order is preserved via `dict.fromkeys` in case `fn` tracks state
        # and depends on the order of the values it evaluates.
        data_length = len(data)
        positions = dict.fromkeys(
            index % data_length
            for index in self.indices
            if -data_length <= index < data_length
        )
        indices_to_remove = (position for position in positions if fn(data[position]))
        for position in sorted(indices_to_remove, reverse=True):
            del data[position]
        return data

    def __eq__(self, other: typing.Any) -> bool:
        return isinstance(other, Index) and sorted(self.indices) == sorted(
            other.indices
        )

    def __str__(self) -> str:
        return "[%s]" % ",".join(str(index) for index in self.indices)

    def __repr__(self) -> str:
        return f"{self.__class__.__name__}(indices={self.indices!r})"

    def _pad_value(self, value: typing.Any) -> None:
        _max = max(self.indices)
        if len(value) <= _max:
            pad = _max - len(value) + 1
            value += [{} for __ in range(pad)]

    def __hash__(self) -> int:
        return hash(self.indices)


class Slice(JSONPath):
    """
    JSONPath matching a slice of an array.

    RFC 9535, section 2.3.4.2: a slice selects nothing from a value
    that is not an array, so ``[:]`` matches every element of an array
    and nothing from an object or a scalar.
    Use ``Wildcard()`` to match every child of an object or an array.
    """

    # True only for a hand-built `Slice()` with no parameters,
    # which is a deprecated wildcard when applied to an object.
    # A slice from a parsed query is never interpreted a wildcard.
    _deprecated_wildcard_use: bool

    def __init__(
        self, start: int | None = None, end: int | None = None, step: int | None = None
    ) -> None:
        self._deprecated_wildcard_use = start is None and end is None and step is None
        if self._deprecated_wildcard_use:
            msg = (
                "A `Slice()` with no parameters will select the members of an object. "
                "In version 2.0.0, `Slice()` will only select items from arrays. "
                "Use `Wildcard()` to select every child."
            )
            warnings.warn(msg, DeprecationWarning, stacklevel=2)
        self.start = start
        self.end = end
        self.step = step

    @classmethod
    def _compliant_constructor(
        cls, start: int | None = None, end: int | None = None, step: int | None = None
    ) -> Slice:
        """
        Create a slice that selects nothing from an object.

        This method solely exists to support instantiation
        without triggering a wildcard-related DeprecationWarning.
        This method will be removed in version 2.0.0.
        """

        node = cls.__new__(cls)
        node.start = start
        node.end = end
        node.step = step
        node._deprecated_wildcard_use = False
        return node

    def find(self, datum: typing.Any) -> list[DatumInContext]:
        datum = DatumInContext.wrap(datum)

        if self._deprecated_wildcard_use and isinstance(datum.value, dict):
            return [
                DatumInContext(
                    value, path=Fields._compliant_constructor(field), context=datum
                )
                for field, value in datum.value.items()
            ]

        if not isinstance(datum.value, (list, tuple)):
            return []

        if self.step == 0:
            return []

        if self.start is None and self.end is None and self.step is None:
            return _find_array_elements(datum)

        return [
            DatumInContext(datum.value[i], path=Index(i), context=datum)
            for i in range(0, len(datum.value))[self.start : self.end : self.step]
        ]

    def update(self, data: typing.Any, val: typing.Any) -> typing.Any:
        for datum in self.find(data):
            datum.path.update(data, val)
        return data

    def filter(self, fn: FilterFunction, data: Container) -> Container:
        if isinstance(data, dict):
            if self._deprecated_wildcard_use:
                for key, value in list(data.items()):
                    if fn(value):
                        del data[key]
            return data

        if not isinstance(data, list) or self.step == 0:
            return data

        # Resolve the positions against the original data before removing anything,
        # then remove them all at once so that the work stays linear.
        positions = range(len(data))[self.start : self.end : self.step]
        positions_to_remove = {position for position in positions if fn(data[position])}
        if positions_to_remove:
            data[:] = [
                value
                for position, value in enumerate(data)
                if position not in positions_to_remove
            ]
        return data

    def __str__(self) -> str:
        elements: list[str] = []

        for i, element in enumerate((self.start, self.end, self.step)):
            if element is None:
                continue
            # Ensure that `elements` contains `i` elements, then append `element`.
            elements.extend([""] * (i - len(elements)))
            elements.append(str(element))

        # If only `start` is present, add a blank element to ensure a trailing ':'.
        if len(elements) == 1:
            elements.append("")

        return f"[{':'.join(elements) or ':'}]"

    def __repr__(self) -> str:
        constructor = self.__class__.__name__
        if (
            not self._deprecated_wildcard_use
            and self.start is None
            and self.end is None
            and self.step is None
        ):
            # Distinguish a slice that selects nothing from an object
            # from the deprecated wildcard.
            constructor += "._compliant_constructor"
        return "{}(start={!r},end={!r},step={!r})".format(
            constructor, self.start, self.end, self.step
        )

    def __eq__(self, other: typing.Any) -> bool:
        return (
            isinstance(other, Slice)
            and other.start == self.start
            and self.end == other.end
            and other.step == self.step
            and self._deprecated_wildcard_use == other._deprecated_wildcard_use
        )

    def __hash__(self) -> int:
        return hash((self.start, self.end, self.step, self._deprecated_wildcard_use))


def _find_array_elements(datum: DatumInContext) -> list[DatumInContext]:
    """Find every element of the array in `datum`."""

    return [
        DatumInContext(child, path=Index(i), context=datum)
        for i, child in enumerate(datum.value)
    ]


def _update_array_elements(data: list[typing.Any], val: typing.Any) -> list[typing.Any]:
    """Update every element of `data` in place."""

    return typing.cast(
        list[typing.Any],
        Index(*range(len(data)))._update_base(data, val, create=False),
    )


def _filter_array_elements(
    fn: FilterFunction, data: list[typing.Any]
) -> list[typing.Any]:
    """Remove from `data`, in place, every element for which `fn` is true."""

    # `fn` sees each original element once, in order,
    # and the work stays linear however many elements are removed.
    kept = [value for value in data if not fn(value)]
    if len(kept) != len(data):
        data[:] = kept
    return data


def _create_list_key(
    dict_: dict[typing.Any, typing.Any],
) -> list[dict[str, typing.Any]]:
    """
    Adds a list to a dictionary by reference and returns the list.

    See `_clean_list_keys()`
    """
    new_list: list[dict[str, typing.Any]] = [{}]
    dict_[LIST_KEY] = new_list
    return new_list


def _clean_list_keys(struct_: Container) -> Container:
    """
    Replace {LIST_KEY: ['foo', 'bar']} with ['foo', 'bar'].

    >>> _clean_list_keys({LIST_KEY: ['foo', 'bar']})
    ['foo', 'bar']

    """
    if isinstance(struct_, list):
        for ind, value in enumerate(struct_):
            struct_[ind] = _clean_list_keys(value)
    elif isinstance(struct_, dict):
        if LIST_KEY in struct_:
            return _clean_list_keys(struct_[LIST_KEY])
        else:
            for key, value in struct_.items():
                struct_[key] = _clean_list_keys(value)
    return struct_


def __getattr__(name: str) -> typing.Any:
    if name in _DEPRECATED_CLASS_NAMES:
        warnings.warn(_deprecated_class_message(name), DeprecationWarning, stacklevel=2)
        return globals()[f"_{name}"]
    raise AttributeError(f"module {__name__!r} has no attribute {name!r}")
