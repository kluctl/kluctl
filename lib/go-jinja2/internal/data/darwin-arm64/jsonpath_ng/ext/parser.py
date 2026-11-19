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

from __future__ import annotations

import typing
import warnings

from .. import Child
from .. import Descendants
from .. import Fields
from .. import Index
from .. import JSONPath
from .. import Root
from .. import This
from .. import lexer
from .. import parser
from .._deprecations import _warn_about_deprecated_syntax

# PLY imports the pre-generated tables by name, which is invisible to PyInstaller.
# Import them explicitly so that they're included in bundles.
from .._ply_tables import ext_lexer_table  # noqa: F401
from .._ply_tables import ext_parser_table  # noqa: F401
from .._ply_types import _Production
from ..exceptions import JsonPathParserError
from . import arithmetic as _arithmetic
from . import filter as _filter
from . import iterable as _iterable
from . import string as _string

if typing.TYPE_CHECKING:
    from .._ply_types import _GenericProduction
    from .._ply_types import _RuleToken
    from .._ply_types import _Value

    _ExtValue: typing.TypeAlias = (
        _Value
        | list[_filter.Expression]
        | list[JSONPath]
        | tuple[JSONPath, bool]
        | list[tuple[JSONPath, bool]]
    )
    _ExtProduction: typing.TypeAlias = _GenericProduction[_ExtValue]


def _conjunction(expressions: list[JSONPath]) -> JSONPath:
    """Combine the expressions of a conjunction into a single expression."""

    if len(expressions) == 1:
        return expressions[0]
    return _filter.And(expressions)


class ExtendedJsonPathLexer(lexer.JsonPathLexer):
    """Custom LALR-lexer for JsonPath"""

    _ply_table_module = "jsonpath_ng._ply_tables.ext_lexer_table"
    literals = lexer.JsonPathLexer.literals + ["?", "@", "+", "*", "/", "-", "!"]
    tokens = (
        ["BOOL", "NULL"]
        + lexer.JsonPathLexer.tokens
        + ["FILTER_OP", "SORT_DIRECTION", "FLOAT", "AND", "OR"]
    )

    t_FILTER_OP = r"=~|==?|<=|>=|!=|<|>"
    # RFC 9535 section 2.3.5.1 logical operators.
    # PLY tries a string rule before it tries a literal, so `&&` is never `&` `&`.
    t_AND = r"&&"
    t_OR = r"\|\|"

    def t_BOOL(self, t: _RuleToken) -> _RuleToken:
        r"true(?![a-zA-Z0-9_@\-])|false(?![a-zA-Z0-9_@\-])"
        t.value = True if t.value == "true" else False
        return t

    def t_SORT_DIRECTION(self, t: _RuleToken) -> _RuleToken:
        r",?\s*(/|\\)"
        t.value = typing.cast(str, t.value)[-1]
        return t

    def t_ID(self, t: _RuleToken) -> _RuleToken:
        r"@?[a-zA-Z_][a-zA-Z0-9_@\-]*"
        # NOTE(sileht): This fixes the ID expression to be
        # able to use @ for `This` like any json query
        t.type = self.reserved_words.get(typing.cast(str, t.value), "ID")
        if t.value == "null":
            t.type = "NULL"
        return t

    def t_FLOAT(self, t: _RuleToken) -> _RuleToken:
        r"-?\d+\.\d+"
        t.value = float(typing.cast(str, t.value))
        return t


class ExtendedJsonPathParser(parser.JsonPathParser):
    """Custom LALR-parser for JsonPath"""

    tokens = ExtendedJsonPathLexer.tokens
    _ply_table_module = "jsonpath_ng._ply_tables.ext_parser_table"

    def __init__(
        self,
        debug: bool | None = None,
        lexer_class: type[lexer.JsonPathLexer] | None = None,
    ) -> None:
        lexer_class = lexer_class or ExtendedJsonPathLexer
        super().__init__(debug, lexer_class)

    def p_jsonpath_operator_jsonpath(self, p: _ExtProduction) -> None:
        """jsonpath : NUMBER operator NUMBER
        | FLOAT operator FLOAT
        | ID operator ID
        | NUMBER operator jsonpath
        | FLOAT operator jsonpath
        | jsonpath operator NUMBER
        | jsonpath operator FLOAT
        | jsonpath operator jsonpath
        """

        # NOTE(sileht): If we have choice between a field or a string we
        # always choice string, because field can be full qualified
        # like $.foo == foo and where string can't.
        for i in [1, 3]:
            item = p[i]
            if isinstance(item, Fields) and len(item.fields) == 1:  # noqa
                p[i] = item.fields[0]

        p[0] = _arithmetic.Operation(p[1], p[2], p[3])

    def p_operator(self, p: _ExtProduction) -> None:
        """operator : '+'
        | '-'
        | '*'
        | '/'
        """
        p[0] = p[1]

    def p_jsonpath_named_operator(self, p: _ExtProduction) -> None:
        "jsonpath : NAMED_OPERATOR"
        name = typing.cast(str, p[1])
        if name == "len":
            p[0] = _iterable.Len()
        elif name == "keys":
            p[0] = _iterable.Keys()
        elif name == "path":
            p[0] = _iterable.Path()
        elif name == "sorted":
            p[0] = _iterable.SortedThis()
        elif name.startswith("split("):
            p[0] = _string.Split(name)
        elif name.startswith("sub("):
            p[0] = _string.Sub(name)
        elif name.startswith("str("):
            p[0] = _string.Str(name)
        else:
            super().p_jsonpath_named_operator(typing.cast(_Production, p))

    def p_expression(self, p: _ExtProduction) -> None:
        """expression : jsonpath
        | jsonpath FILTER_OP ID
        | jsonpath FILTER_OP FLOAT
        | jsonpath FILTER_OP NUMBER
        | jsonpath FILTER_OP BOOL
        | jsonpath FILTER_OP NULL
        | jsonpath FILTER_OP singular_query
        """
        if len(p) == 2:
            left, op, right = p[1], None, None
        else:
            __, left, op, right = p
            if p.slice[3].type == "singular_query" and op == "=~":
                # The regex operator is not part of RFC 9535
                # and takes a literal pattern.
                raise JsonPathParserError(
                    "The regex operator requires a literal pattern, not a query"
                )
            if p.slice[3].type == "ID" and not getattr(p.slice[3], "quoted", False):
                # RFC 9535 has no unquoted strings; `jsonpath-ng` has always
                # treated a bare word as the string of the same name.
                _warn_about_deprecated_syntax(
                    "An unquoted word as a comparison operand is deprecated. "
                    "Support for unquoted words will be removed in version 2.0.0. "
                    f"Write `{right!r}` instead."
                )
            if p.slice[3].type == "NULL":
                # The literal `null` compares as JSON null, which `Expression`
                # represents as `None`. The regex operator is not part of RFC 9535
                # and has always taken the bare word as the text of its pattern.
                right = "null" if op == "=~" else None
        p[0] = _filter.Expression(left, op, right)

    def p_singular_query_identifier(self, p: _ExtProduction) -> None:
        """singular_query : '@'
        | '$'
        """
        # RFC 9535 section 2.3.5.1: a comparison operand is a *singular* query,
        # which starts with `@` or `$` and has only name and index segments.
        p[0] = This() if p[1] == "@" else Root()

    def p_singular_query_name(self, p: _ExtProduction) -> None:
        "singular_query : singular_query '.' singular_name"
        p[0] = Child(
            typing.cast(JSONPath, p[1]),
            Fields._compliant_constructor(typing.cast(str, p[3])),
        )

    def p_singular_query_bracketed_name(self, p: _ExtProduction) -> None:
        "singular_query : singular_query '[' ID ']'"
        p[0] = Child(
            typing.cast(JSONPath, p[1]),
            Fields._compliant_constructor(typing.cast(str, p[3])),
        )

    def p_singular_query_index(self, p: _ExtProduction) -> None:
        "singular_query : singular_query '[' NUMBER ']'"
        p[0] = Child(typing.cast(JSONPath, p[1]), Index(typing.cast(int, p[3])))

    def p_singular_name(self, p: _ExtProduction) -> None:
        """singular_name : ID
        | NULL
        | BOOL
        """
        # `null`, `true`, and `false` are literals in filter comparisons
        # but are still valid field names.
        if isinstance(p[1], bool):
            p[0] = "true" if p[1] else "false"
        else:
            p[0] = p[1]

    def p_fields_null(self, p: _ExtProduction) -> None:
        "fields : NULL"
        # `null` is a literal in filter comparisons but is still a valid field name.
        p[0] = [p[1]]

    def p_fields_bool(self, p: _ExtProduction) -> None:
        "fields : BOOL"
        # `true` and `false` are literals in filter comparisons but are still valid
        # field names. The lexer has already converted the text to a Python bool.
        p[0] = ["true" if p[1] else "false"]

    def p_expression_not(self, p: _ExtProduction) -> None:
        "expression : '!' jsonpath"
        p[0] = _filter.Expression(p[2], p[1], None)

    # RFC 9535 section 2.3.5.1: `&&` binds more tightly than `||`.
    #
    #   expressions      (`||` of one or more conjunctions)
    #   and_expressions  (`&&` of one or more operands)
    #   operand          (an expression, or a parenthesized `expressions`)
    #
    # A conjunction is a list of expressions, as it has always been in `Filter`.
    # The parser creates an `Or` node for `||`, and an `And` node only for a
    # conjunction that is an operand of `||`.

    def p_expressions_and_expressions(self, p: _ExtProduction) -> None:
        "expressions : and_expressions"
        p[0] = p[1]

    def p_expressions_or(self, p: _ExtProduction) -> None:
        "expressions : expressions OR and_expressions"
        left = typing.cast(list[JSONPath], p[1])
        right = _conjunction(typing.cast(list[JSONPath], p[3]))
        if len(left) == 1 and isinstance(left[0], _filter.Or):
            # Extend the `Or` that the previous step created, which nothing else
            # references yet, instead of copying its operands for every term.
            left[0].operands.extend(
                right.operands if isinstance(right, _filter.Or) else [right]
            )
            p[0] = left
        else:
            p[0] = [_filter.Or([_conjunction(left), right])]

    def p_and_expressions_operand(self, p: _ExtProduction) -> None:
        "and_expressions : operand"
        p[0] = p[1]

    def p_and_expressions_and(self, p: _ExtProduction) -> None:
        "and_expressions : and_expressions AND operand"
        p[0] = typing.cast(list[JSONPath], p[1]) + typing.cast(list[JSONPath], p[3])

    def p_and_expressions_ampersand(self, p: _ExtProduction) -> None:
        "and_expressions : and_expressions '&' operand"
        _warn_about_deprecated_syntax(
            "The `&` operator in filters is deprecated. "
            "It will be removed in version 2.0.0. "
            "Use `&&` instead."
        )
        p[0] = typing.cast(list[JSONPath], p[1]) + typing.cast(list[JSONPath], p[3])

    def p_operand_expression(self, p: _ExtProduction) -> None:
        "operand : expression"
        p[0] = [p[1]]

    def p_operand_parens(self, p: _ExtProduction) -> None:
        "operand : '(' expressions ')'"
        p[0] = p[2]

    def p_filter(self, p: _ExtProduction) -> None:
        "filter : '?' expressions"
        p[0] = _filter.Filter(p[2])

    def p_jsonpath_filter(self, p: _ExtProduction) -> None:
        "jsonpath : jsonpath '[' filter ']'"
        p[0] = Child(typing.cast(JSONPath, p[1]), typing.cast(JSONPath, p[3]))

    def p_jsonpath_descendant_filter(self, p: _ExtProduction) -> None:
        "jsonpath : jsonpath DOUBLEDOT '[' filter ']'"
        p[0] = Descendants(typing.cast(JSONPath, p[1]), typing.cast(JSONPath, p[4]))

    def p_sort(self, p: _ExtProduction) -> None:
        "sort : SORT_DIRECTION jsonpath"
        p[0] = (p[2], p[1] != "/")

    def p_sorts_sort(self, p: _ExtProduction) -> None:
        "sorts : sort"
        p[0] = [p[1]]

    def p_sorts_comma(self, p: _ExtProduction) -> None:
        "sorts : sorts sorts"
        p[0] = typing.cast(list[tuple[JSONPath, bool]], p[1]) + typing.cast(
            list[tuple[JSONPath, bool]], p[2]
        )

    def p_jsonpath_sort(self, p: _ExtProduction) -> None:
        "jsonpath : jsonpath '[' sorts ']'"
        sort = _iterable.SortedThis(p[3])
        p[0] = Child(typing.cast(JSONPath, p[1]), sort)

    def p_jsonpath_this(self, p: _ExtProduction) -> None:
        "jsonpath : '@'"
        p[0] = This()

    precedence = (
        [
            ("left", "+", "-"),
            ("left", "*", "/"),
        ]
        + parser.JsonPathParser.precedence
        + [
            ("nonassoc", "ID"),
        ]
    )


def parse(path: str, debug: bool | None = None) -> JSONPath:
    return ExtendedJsonPathParser(debug=debug).parse(path)


def __getattr__(name: str) -> object:
    if name == "ExtentedJsonPathParser":
        msg = (
            "ExtentedJsonPathParser is a deprecated name. "
            "It will be removed in version 2.0.0. "
            "Please use 'ExtendedJsonPathParser' instead."
        )
        warnings.warn(msg, DeprecationWarning, stacklevel=2)
        return ExtendedJsonPathParser
    raise AttributeError(f"Module {__name__!r} has no attribute {name}")
