"""
Mock the unannotated `ply` classes for type-checking purposes.

`ply` will eventually be replaced with a supported lexer/parser,
so annotating the vendored library is not considered worthwhile.
"""

from __future__ import annotations

import typing

from jsonpath_ng.jsonpath import JSONPath

_ValueT_co = typing.TypeVar("_ValueT_co", covariant=True)

# `_GenericProduction.slice` shadows the builtin within the class body.
_BuiltinSlice: typing.TypeAlias = slice


class _Lexer:
    # Mock the ply lexer class for type annotations.
    input: typing.Callable[[str], None]
    latest_newline: int
    lexpos: int
    lineno: int
    pop_state: typing.Callable[[], None]
    push_state: typing.Callable[[str], None]
    string_value: str | None
    token: typing.Callable[[], _Token | None]


class _RuleToken:
    # Mock the ply token class, as seen by the `t_*` rule functions.
    # `col` is only set later, by `JsonPathLexer.tokenize()`.
    lexer: _Lexer
    lexpos: int
    lineno: int
    quoted: bool
    type: str
    value: float | str | None


class _Token:
    # Mock the ply token class, as yielded by `JsonPathLexer.tokenize()`.
    # `lexer` is not included because ply only sets it for tokens
    # produced by a rule function, not for literals like `.` or `[`.
    col: int
    lexpos: int
    lineno: int
    type: str
    value: float | str | None


class _Symbol:
    # Mock the ply grammar-symbol classes (`LexToken` and `YaccSymbol`),
    # as seen through `YaccProduction.slice`.
    type: str


class _TokenSource(typing.Protocol):
    def token(self) -> _Token | None: ...


class _Parser:
    # Mock the ply parser class for type annotations.
    def parse(self, lexer: _TokenSource) -> JSONPath:
        raise NotImplementedError("For type annotations only")


_Value: typing.TypeAlias = JSONPath | list[int] | list[str] | int | float | str | None


class _GenericProduction(typing.Generic[_ValueT_co]):
    # Mock the ply production class for type annotations.
    #
    # The type parameter is the set of values that a grammar's symbols can have.
    # Parsers that extend the grammar can widen it.
    lexpos: typing.Callable[[int], int]
    lineno: typing.Callable[[int], int]
    slice: list[_Symbol]

    @typing.overload
    def __getitem__(self, i: int) -> _ValueT_co: ...

    @typing.overload
    def __getitem__(self, i: _BuiltinSlice) -> list[_ValueT_co]: ...

    def __getitem__(self, i: int | _BuiltinSlice) -> _ValueT_co | list[_ValueT_co]:
        raise NotImplementedError("For type annotations only")

    def __iter__(self) -> typing.Iterator[_ValueT_co]:
        raise NotImplementedError("For type annotations only")

    def __len__(self) -> int:
        raise NotImplementedError("For type annotations only")

    def __setitem__(self, i: int, value: object) -> None:
        raise NotImplementedError("For type annotations only")


_Production: typing.TypeAlias = _GenericProduction[_Value]
