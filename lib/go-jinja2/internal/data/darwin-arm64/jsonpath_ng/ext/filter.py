#
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
# License for the specific language governing permissions and limitations
# under the License.

import operator
import re
import typing as t
import warnings

from .. import Child
from .. import DatumInContext
from .. import Descendants
from .. import Fields
from .. import Index
from .. import JSONPath
from .. import Root
from .. import This
from .arithmetic import Operation

StringOperator: t.TypeAlias = t.Literal["!=", "==", "=", "<=", "<", ">=", ">"]

OPERATOR_MAP: dict[
    StringOperator | t.Literal["=~"], t.Callable[[t.Any, t.Any], bool]
] = {
    "!=": operator.ne,
    "==": operator.eq,
    "=": operator.eq,
    "<=": operator.le,
    "<": operator.lt,
    ">=": operator.ge,
    ">": operator.gt,
    "=~": lambda a, b: True if isinstance(a, str) and re.search(b, a) else False,
}

# RFC 9535 section 2.3.5.2.2: `a != b` is true if and only if `a == b` is false,
# and `a <= b` is true if and only if `a < b` or `a == b`.
#
# Only numbers and strings are ordered, so for JSON null and for booleans
# the ordering comparisons reduce to equality.
# Each function receives whether the two values are equal.
EQUALITY_ONLY_OPERATOR_MAP: dict[str, t.Callable[[t.Any], bool]] = {
    "!=": operator.not_,
    "==": bool,
    "=": bool,
    "<=": bool,
    ">=": bool,
    "<": lambda equal: False,
    ">": lambda equal: False,
}


def _is_number(value: t.Any) -> bool:
    """
    Determine if a value is a number.

    In Python, bool is a subclass of int and would satisfy `isinstance(..., int)`.
    It's therefore necessary to restrict bool when determining if a value is an int,
    and this function exists to ensure co.
    """

    return isinstance(value, (int, float)) and not isinstance(value, bool)


def _equal(left: t.Any, right: t.Any) -> bool:
    """Compare two JSON values as defined by RFC 9535 section 2.3.5.2.2.

    Values of different types are never equal,
    so booleans must be explicitly handled to prevent comparing bools and ints.
    For example, `True` and `1` must never compare as equal.
    """

    pairs = [(left, right)]
    while pairs:
        left, right = pairs.pop()
        if isinstance(left, bool) or isinstance(right, bool):
            # `is` is used here to prevent `True` from comparing as equal to `1`.
            # It works because bools are singletons in Python.
            if left is not right:
                return False
        elif isinstance(left, list) and isinstance(right, list):
            if len(left) != len(right):
                return False
            pairs.extend(zip(left, right))
        elif isinstance(left, dict) and isinstance(right, dict):
            if left.keys() != right.keys():
                return False
            pairs.extend((value, right[key]) for key, value in left.items())
        elif isinstance(left, (list, dict)) or isinstance(right, (list, dict)):
            return False
        elif left != right:
            return False
    return True


def _compare(op: StringOperator | t.Literal["=~"], left: t.Any, right: t.Any) -> bool:
    """Compare two JSON values as defined by RFC 9535 section 2.3.5.2.2.

    Only two numbers or two strings are ordered.
    For all other values, the ordering comparisons reduce to equality.
    """

    if (_is_number(left) and _is_number(right)) or (
        isinstance(left, str) and isinstance(right, str)
    ):
        return OPERATOR_MAP[op](left, right)
    return EQUALITY_ONLY_OPERATOR_MAP[op](_equal(left, right))


def _query_to_str(path: t.Any, leading: bool = True) -> str:
    """Serialize a query that is used inside a filter.

    RFC 9535 section 2.3.5 starts a filter's relative query with `@`,
    so the current node is written as `@` where the query begins.
    Elsewhere, the current node can only be written as "`this`",
    which is not RFC 9535 syntax and should not be promoted
    to a position where it looks like it is.

    Bracketed segments are written without a leading `.`,
    as required by the singular queries that can be compared to a value.
    """

    if isinstance(path, This):
        return "@" if leading else str(path)
    if isinstance(path, Child):
        left = _query_to_str(path.left, leading)
        right = _query_to_str(path.right, False)
        if right.startswith("["):
            # Indices, slices, filters, and sorts are bracketed, with no period.
            return f"{left}{right}"
        return f"{left}.{right}"
    if isinstance(path, Descendants):
        return (
            f"({_query_to_str(path.left, leading)}..{_query_to_str(path.right, False)})"
        )
    if isinstance(path, Operation):
        left = _query_to_str(path.left)
        right = _query_to_str(path.right)
        return f"{left} {path.op_symbol} {right}"
    return str(path)


class Filter(JSONPath):
    """The JSONQuery filter"""

    def __init__(self, expressions) -> None:
        if not expressions:
            msg = (
                "Creating a Filter with no expressions is deprecated. "
                "It will raise an exception in version 2.0.0."
            )
            warnings.warn(msg, DeprecationWarning, stacklevel=2)
        self.expressions = list(expressions)

    def _references_root(self) -> bool:
        """Identify whether the filter expressions reference the root identifier."""

        paths_to_visit: list[JSONPath] = list(self.expressions)

        while paths_to_visit:
            path = paths_to_visit.pop()
            if isinstance(path, Root):
                return True

            if isinstance(path, Filter):
                paths_to_visit.extend(path.expressions)
            elif isinstance(path, Expression):
                paths_to_visit.append(path.target)
                if isinstance(path.value, JSONPath):
                    paths_to_visit.append(path.value)
            elif isinstance(path, _BooleanOperation):
                paths_to_visit.extend(path.operands)
            else:
                if (left := getattr(path, "left", None)) is not None:
                    paths_to_visit.append(left)
                if (right := getattr(path, "right", None)) is not None:
                    paths_to_visit.append(right)

        return False

    def find(self, datum):
        datum = DatumInContext.wrap(datum)

        if isinstance(datum.value, dict):
            candidates = datum.value.items()
            make_path = Fields._compliant_constructor
        elif isinstance(datum.value, list):
            candidates = enumerate(datum.value)
            make_path = Index
        else:
            return []

        # RFC 9535 section 1.4.1: `$` is the root of the query argument,
        # so an expression that uses it must be evaluated on a candidate
        # that is linked to its parent.
        if self._references_root():
            results = []
            for key, value in candidates:
                candidate = DatumInContext(value, path=make_path(key), context=datum)
                if all(expression.find(candidate) for expression in self.expressions):
                    results.append(candidate)
            return results

        # Otherwise, avoid creating candidates that will not match.
        return [
            DatumInContext(value, path=make_path(key), context=datum)
            for key, value in candidates
            if all(expression.find(value) for expression in self.expressions)
        ]

    @staticmethod
    def _datum(data, root):
        """Wrap *data* so that `$` resolves to *root*."""

        if data is root:
            return DatumInContext(data)
        return DatumInContext(data, context=DatumInContext(root))

    def filter(self, fn, data):
        return self._filter_with_root(fn, data, data)

    def _filter_with_root(self, fn, data, root):
        # Reverse the order to ensure indices are preserved upon removal.
        for datum in reversed(self.find(self._datum(data, root))):
            datum.path.filter(fn, data)
        return data

    def update(self, data, val):
        return self._update_with_root(data, val, data)

    def _update_with_root(self, data, val, root):
        if type(data) is list:
            if self._references_root():
                # Evaluate `$` against the root, as `find()` does.
                parent = self._datum(data, root)
                for index, item in enumerate(data):
                    candidate = DatumInContext(item, path=Index(index), context=parent)
                    if all(
                        expression.find(candidate) for expression in self.expressions
                    ):
                        self._set(data, index, val)
            else:
                for index, item in enumerate(data):
                    if all(expression.find(item) for expression in self.expressions):
                        self._set(data, index, val)
        return data

    @staticmethod
    def _set(data, index, val):
        if callable(val):
            val(data[index], data, index)
        else:
            data[index] = val

    def __repr__(self):
        return f"{self.__class__.__name__}({self.expressions!r})"

    def __str__(self) -> str:
        # A filter's expressions form a conjunction, which binds more tightly than `||`.
        # A lone disjunction needs no parentheses.
        group = len(self.expressions) > 1
        parts = (
            (
                f"({expression})"
                if group and isinstance(expression, Or)
                else str(expression)
            )
            for expression in self.expressions
        )
        return f"[?{' && '.join(parts)}]"

    def __eq__(self, other):
        return isinstance(other, Filter) and self.expressions == other.expressions


class Expression(JSONPath):
    """The JSONQuery expression"""

    def __init__(self, target, op, value) -> None:
        self.target = target
        self.op = op
        self.value = value

    def find(self, datum):
        wrapped_datum = DatumInContext.wrap(datum)
        found = self.target.find(wrapped_datum)

        if self.op == "!":
            return [] if found else [wrapped_datum]

        if isinstance(self.value, JSONPath):
            return self._find_with_query_operand(wrapped_datum, found)

        # A comparison with the literal `null` has `None` as its value, which is
        # otherwise only the case for the existence test and negation handled above.
        null_comparison = self.op is not None and self.value is None
        if not found:
            # RFC 9535 section 2.3.5.2.2: an empty nodelist is not equal to any value,
            # including `null`, so only `!=` is true when the member is missing.
            return [wrapped_datum] if self.op == "!=" else []
        if self.op is None:
            return found

        result = []
        for data in found:
            value = data.value
            if null_comparison:
                if EQUALITY_ONLY_OPERATOR_MAP[self.op](value is None):
                    result.append(data)
                continue
            if self.op != "=~" and (
                isinstance(value, bool) or isinstance(self.value, bool)
            ):
                # A boolean is never equal to a number according to RFC 9535.
                # However, in Python, bool is a subclass of int and is comparable
                # via equality (`False == 0`, `True == 1`).
                # Therefore, an identity comparison (`is`) is used,
                # which relies on the fact that bools in Python are singletons.
                if EQUALITY_ONLY_OPERATOR_MAP[self.op](value is self.value):
                    result.append(data)
                continue
            # RFC 9535 section 2.3.5.2.2: a string is never equal to a number,
            # and only two numbers or two strings are ordered.
            if self.op == "=~":
                matched = OPERATOR_MAP["=~"](value, self.value)
            else:
                matched = _compare(self.op, value, self.value)
            if matched:
                result.append(data)

        return result

    def _find_with_query_operand(self, datum, found):
        """Compare the values found by the target to those found by the query operand.

        RFC 9535 section 2.3.5.2.2 says that two empty nodelists are equal,
        and that an empty nodelist is not equal to any value.
        """

        values = [match.value for match in self.value.find(datum)]
        if not found or not values:
            # Nothing is equal to `null`, to `false`, or to anything else.
            if EQUALITY_ONLY_OPERATOR_MAP[self.op](not found and not values):
                return found or [datum]
            return []

        return [
            match
            for match in found
            if any(_compare(self.op, match.value, value) for value in values)
        ]

    def __eq__(self, other):
        return (
            isinstance(other, Expression)
            and self.target == other.target
            and self.op == other.op
            and self.value == other.value
        )

    def __repr__(self):
        if self.op is None:
            return f"{self.__class__.__name__}({self.target!r})"
        else:
            return "{}({!r} {} {!r})".format(
                self.__class__.__name__, self.target, self.op, self.value
            )

    def __str__(self):
        target = _query_to_str(self.target)
        if self.op is None:
            return target
        if self.op == "!":
            return f"!{target}"

        value = self.value
        if isinstance(value, JSONPath):
            rendered = _query_to_str(value)
        elif value is None:
            rendered = "null"
        elif isinstance(value, bool):
            rendered = "true" if value else "false"
        elif isinstance(value, str):
            # Always quote strings.
            # This ensures that strings like "true", "10", and "a b" can parse again.
            rendered = repr(value)
        else:
            rendered = str(value)
        return f"{target} {self.op} {rendered}"


class _BooleanOperation(JSONPath):
    """The base class for the logical operators of an RFC 9535 filter.

    The operands are stored in a flat list, rather than as nested binary nodes,
    so that a long chain like `a || b || c || ...` does not become a deeply
    nested tree. `||` and `&&` are associative, so no meaning is lost.
    """

    op: t.ClassVar[str]

    def __init__(self, operands: t.Iterable[JSONPath]) -> None:
        self.operands: list[JSONPath] = []
        for operand in operands:
            if type(operand) is type(self):
                # Operands are already flat, so one level is all that can nest.
                self.operands.extend(operand.operands)  # type: ignore[attr-defined]
            else:
                self.operands.append(operand)

    def __eq__(self, other: object) -> bool:
        return (
            type(self) is type(other)
            and self.operands == other.operands  # type: ignore[attr-defined]
        )

    def __repr__(self) -> str:
        return f"{self.__class__.__name__}({self.operands!r})"

    def _operand_to_str(self, operand: JSONPath) -> str:
        raise NotImplementedError

    def __str__(self) -> str:
        return f" {self.op} ".join(
            self._operand_to_str(operand) for operand in self.operands
        )


class And(_BooleanOperation):
    """The RFC 9535 logical AND, `&&`, of filter expressions.

    The parser only creates this for a conjunction that is an operand of `||`.
    Elsewhere, a conjunction is a `Filter`'s list of expressions.
    """

    op = "&&"

    def find(self, datum):
        if all(operand.find(datum) for operand in self.operands):
            return [DatumInContext.wrap(datum)]
        return []

    def _operand_to_str(self, operand: JSONPath) -> str:
        # `||` binds less tightly than `&&`.
        return f"({operand})" if isinstance(operand, Or) else str(operand)


class Or(_BooleanOperation):
    """The RFC 9535 logical OR, `||`, of filter expressions."""

    op = "||"

    def find(self, datum):
        if any(operand.find(datum) for operand in self.operands):
            return [DatumInContext.wrap(datum)]
        return []

    def _operand_to_str(self, operand: JSONPath) -> str:
        # `&&` binds more tightly, so an `And` operand needs no parentheses.
        return str(operand)
