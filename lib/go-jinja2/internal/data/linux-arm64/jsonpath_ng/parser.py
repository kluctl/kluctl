from __future__ import annotations

import logging
import sys
import typing
import warnings

import jsonpath_ng._ply.yacc

# PLY imports the pre-generated table by name, which is invisible to PyInstaller.
# Import it explicitly so that it's included in bundles.
import jsonpath_ng._ply_tables.parser_table  # noqa: F401
from jsonpath_ng._deprecations import _warn_about_deprecated_syntax
from jsonpath_ng.exceptions import JsonPathParserError
from jsonpath_ng.jsonpath import Child
from jsonpath_ng.jsonpath import Descendants
from jsonpath_ng.jsonpath import Fields
from jsonpath_ng.jsonpath import Index
from jsonpath_ng.jsonpath import JSONPath
from jsonpath_ng.jsonpath import Parent
from jsonpath_ng.jsonpath import Root
from jsonpath_ng.jsonpath import Slice
from jsonpath_ng.jsonpath import This
from jsonpath_ng.jsonpath import Where
from jsonpath_ng.jsonpath import WhereNot
from jsonpath_ng.jsonpath import Wildcard
from jsonpath_ng.jsonpath import _Intersect
from jsonpath_ng.jsonpath import _Union
from jsonpath_ng.lexer import JsonPathLexer

if typing.TYPE_CHECKING:
    from ._ply_types import _Parser
    from ._ply_types import _Production
    from ._ply_types import _Token

logger = logging.getLogger(__name__)


def parse(string: str) -> JSONPath:
    return JsonPathParser().parse(string)


class JsonPathParser:
    """
    An LALR-parser for JsonPath
    """

    tokens = JsonPathLexer.tokens

    # The pre-generated LALR table; see `assets/generate_ply_tables.py`.
    _ply_table_module = "jsonpath_ng._ply_tables.parser_table"

    def __init__(
        self, debug: bool | None = None, lexer_class: type[JsonPathLexer] | None = None
    ) -> None:
        if debug is not None:
            msg = (
                "The `debug` parameter is deprecated. "
                "It no longer has any effect and will be removed in v2.0.0."
            )
            warnings.warn(msg, DeprecationWarning, stacklevel=2)

        self.lexer_class = (
            lexer_class or JsonPathLexer
        )  # Crufty but works around statefulness in PLY

        # Load the pre-generated parse table. Nothing is generated or written.
        self.parser: _Parser = jsonpath_ng._ply.yacc.yacc(  # type: ignore[no-untyped-call]
            module=self,
            debug=False,
            tabmodule=self._ply_table_module,
            optimize=True,
            write_tables=False,
            start="jsonpath",
            errorlog=logger,
        )

    def parse(self, string: str, lexer: JsonPathLexer | None = None) -> JSONPath:
        lexer = lexer or self.lexer_class()
        return self.parse_token_stream(lexer.tokenize(string))

    def parse_token_stream(self, token_iterator: typing.Iterator[_Token]) -> JSONPath:
        return self.parser.parse(lexer=IteratorToTokenStream(token_iterator))

    # ===================== PLY Parser specification =====================

    precedence: list[tuple[str, str] | tuple[str, str, str]] = [
        ("left", ","),
        ("left", "DOUBLEDOT"),
        ("left", "."),
        ("left", "|"),
        ("left", "&"),
        ("left", "WHERE"),
        ("left", "WHERENOT"),
    ]

    def p_error(self, t: _Token | None) -> typing.Never:
        if t is None:
            raise JsonPathParserError("Parse error near the end of string!")
        raise JsonPathParserError(
            "Parse error at %s:%s near token %s (%s)"
            % (t.lineno, t.col, t.value, t.type)
        )

    def p_jsonpath_binop(self, p: _Production) -> None:
        """jsonpath : jsonpath '.' jsonpath
        | jsonpath DOUBLEDOT jsonpath
        | jsonpath WHERE jsonpath
        | jsonpath WHERENOT jsonpath
        | jsonpath '|' jsonpath
        | jsonpath '&' jsonpath"""
        op = p[2]
        left = typing.cast(JSONPath, p[1])
        right = typing.cast(JSONPath, p[3])

        if op == ".":
            p[0] = Child(left, right)
        elif op == "..":
            p[0] = Descendants(left, right)
        elif op == "where":
            p[0] = Where(left, right)
        elif op == "wherenot":
            p[0] = WhereNot(left, right)
        elif op == "|":
            _warn_about_deprecated_syntax(
                "The `|` (union) operator is deprecated. "
                "It will be removed in version 2.0.0."
            )
            p[0] = _Union(left, right)
        elif op == "&":
            _warn_about_deprecated_syntax(
                "The `&` (intersection) operator is deprecated. "
                "It will be removed in version 2.0.0."
            )
            p[0] = _Intersect(left, right)

    def p_jsonpath_fields(self, p: _Production) -> None:
        "jsonpath : fields_or_any"
        if isinstance(p[1], Wildcard):
            p[0] = p[1]
        else:
            p[0] = Fields._compliant_constructor(*typing.cast(list[str], p[1]))

    def p_jsonpath_named_operator(self, p: _Production) -> None:
        "jsonpath : NAMED_OPERATOR"
        if p[1] == "this":
            p[0] = This()
        elif p[1] == "parent":
            p[0] = Parent()
        else:
            raise JsonPathParserError(
                "Unknown named operator `%s` at %s:%s"
                % (p[1], p.lineno(1), p.lexpos(1))
            )

    def p_jsonpath_root(self, p: _Production) -> None:
        "jsonpath : '$'"
        p[0] = Root()

    def p_jsonpath_idx(self, p: _Production) -> None:
        "jsonpath : '[' idx ']'"
        p[0] = Index(*typing.cast(list[int], p[2]))

    def p_jsonpath_slice(self, p: _Production) -> None:
        "jsonpath : '[' slice ']'"
        p[0] = typing.cast(JSONPath, p[2])

    def p_jsonpath_fieldbrackets(self, p: _Production) -> None:
        "jsonpath : '[' fields ']'"
        p[0] = Fields._compliant_constructor(*typing.cast(list[str], p[2]))

    def p_jsonpath_child_fieldbrackets(self, p: _Production) -> None:
        "jsonpath : jsonpath '[' fields ']'"
        p[0] = Child(
            typing.cast(JSONPath, p[1]),
            Fields._compliant_constructor(*typing.cast(list[str], p[3])),
        )

    def p_jsonpath_child_idxbrackets(self, p: _Production) -> None:
        "jsonpath : jsonpath '[' idx ']'"
        p[0] = Child(typing.cast(JSONPath, p[1]), Index(*typing.cast(list[int], p[3])))

    def p_jsonpath_child_slicebrackets(self, p: _Production) -> None:
        "jsonpath : jsonpath '[' slice ']'"
        p[0] = Child(typing.cast(JSONPath, p[1]), typing.cast(JSONPath, p[3]))

    def p_jsonpath_parens(self, p: _Production) -> None:
        "jsonpath : '(' jsonpath ')'"
        p[0] = typing.cast(JSONPath, p[2])

    # An unquoted '*' is the wildcard selector (RFC 9535, section 2.3.2);
    # a quoted '*' is a field name and arrives here as a `fields` list.
    def p_fields_or_any(self, p: _Production) -> None:
        """fields_or_any : fields
        | '*'
        | NUMBER"""
        p1 = p[1]
        if p1 == "*":
            p[0] = Wildcard()
        elif isinstance(p1, int):
            p[0] = [str(p1)]
        else:
            p[0] = typing.cast(list[int | str], p1)

    def p_fields_id(self, p: _Production) -> None:
        "fields : ID"
        p[0] = [typing.cast(str, p[1])]

    def p_fields_comma(self, p: _Production) -> None:
        "fields : fields ',' fields"
        p[0] = typing.cast(list[str] | list[int], p[1]) + typing.cast(
            list[str] | list[int], p[3]
        )

    def p_idx(self, p: _Production) -> None:
        "idx : NUMBER"
        p[0] = [typing.cast(int, p[1])]

    def p_idx_comma(self, p: _Production) -> None:
        "idx : idx ',' idx"
        p[0] = typing.cast(list[int], p[1]) + typing.cast(list[int], p[3])

    def p_slice_any(self, p: _Production) -> None:
        "slice : '*'"
        p[0] = Wildcard()

    def p_slice(self, p: _Production) -> None:  # Currently does not support `step`
        """slice : maybe_int ':' maybe_int
        | maybe_int ':' maybe_int ':' maybe_int"""
        p[0] = Slice._compliant_constructor(*typing.cast(list[int | None], p[1::2]))

    def p_maybe_int(self, p: _Production) -> None:
        """maybe_int : NUMBER
        | empty"""
        p[0] = typing.cast(int | None, p[1])

    def p_empty(self, p: _Production) -> None:
        "empty :"
        p[0] = None


class IteratorToTokenStream:
    def __init__(self, iterator: typing.Iterator[_Token]) -> None:
        self.iterator = iterator

    def token(self) -> _Token | None:
        try:
            return next(self.iterator)
        except StopIteration:
            return None


if __name__ == "__main__":
    logging.basicConfig()
    parser = JsonPathParser()
    print(parser.parse(sys.stdin.read()))
