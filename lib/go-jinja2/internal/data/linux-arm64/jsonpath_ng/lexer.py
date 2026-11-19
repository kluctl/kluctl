from __future__ import annotations

import logging
import sys
import typing
import warnings

import jsonpath_ng._ply.lex

# PLY imports the pre-generated table by name, which is invisible to PyInstaller.
# Import it explicitly so that it's included in bundles.
import jsonpath_ng._ply_tables.lexer_table  # noqa: F401
from jsonpath_ng.exceptions import JsonPathLexerError

if typing.TYPE_CHECKING:
    from jsonpath_ng._ply_types import _Lexer
    from jsonpath_ng._ply_types import _RuleToken
    from jsonpath_ng._ply_types import _Token

logger = logging.getLogger(__name__)


class JsonPathLexer:
    """
    A Lexical analyzer for JsonPath.
    """

    # The pre-generated lexer table; see `assets/generate_ply_tables.py`.
    _ply_table_module = "jsonpath_ng._ply_tables.lexer_table"

    def __init__(self, debug: bool | None = None) -> None:
        if debug is not None:
            msg = (
                "The `debug` parameter is deprecated. "
                "It no longer has any effect and will be removed in version 2.0.0."
            )
            warnings.warn(msg, DeprecationWarning, stacklevel=2)

    def tokenize(self, string: str) -> typing.Iterator[_Token]:
        """
        Maps a string to an iterator over tokens. In other words: [char] -> [token]
        """

        new_lexer: _Lexer = jsonpath_ng._ply.lex.lex(  # type: ignore[no-untyped-call]
            module=self,
            debug=False,
            optimize=True,
            lextab=self._ply_table_module,
            errorlog=logger,
        )
        new_lexer.latest_newline = 0
        new_lexer.string_value = None
        new_lexer.input(string)

        while True:
            t = new_lexer.token()
            if t is None:
                break
            t.col = t.lexpos - new_lexer.latest_newline
            yield t

        if new_lexer.string_value is not None:
            raise JsonPathLexerError("Unexpected EOF in string literal or identifier")

    # ============== PLY Lexer specification ==================
    #
    # This probably should be private but:
    #   - the parser requires access to `tokens`
    #     (perhaps they should be defined in a third, shared dependency)
    #   - things like `literals` might be a legitimate part of the public interface.
    #
    # Anyhow, it is pythonic to give some rope to hang oneself with :-)

    literals = ["*", ".", "[", "]", "(", ")", "$", ",", ":", "|", "&", "~"]

    reserved_words = {
        "where": "WHERE",
        "wherenot": "WHERENOT",
    }

    tokens = ["DOUBLEDOT", "NUMBER", "ID", "NAMED_OPERATOR"] + list(
        reserved_words.values()
    )

    states = [
        ("singlequote", "exclusive"),
        ("doublequote", "exclusive"),
        ("backquote", "exclusive"),
    ]

    # Normal lexing, rather easy
    t_DOUBLEDOT = r"\.\."
    t_ignore = " \t"

    def t_ID(self, t: _RuleToken) -> _RuleToken:
        # CJK: [\u4E00-\u9FA5]
        # EMOJI: [\U0001F600-\U0001F64F]
        r"([a-zA-Z_@]|[\u4E00-\u9FA5]|[\U0001F600-\U0001F64F])([a-zA-Z0-9_@\-]|[\u4E00-\u9FA5]|[\U0001F600-\U0001F64F])*"
        t.type = self.reserved_words.get(typing.cast(str, t.value), "ID")
        return t

    def t_NUMBER(self, t: _RuleToken) -> _RuleToken:
        r"-?\d+"
        t.value = int(typing.cast(str, t.value))
        return t

    # Single-quoted strings
    t_singlequote_ignore = ""

    def t_singlequote(self, t: _RuleToken) -> None:
        r"'"
        t.lexer.string_value = ""
        t.lexer.push_state("singlequote")

    def t_singlequote_content(self, t: _RuleToken) -> None:
        r"[^'\\]+"
        assert isinstance(t.lexer.string_value, str)
        t.lexer.string_value += typing.cast(str, t.value)

    def t_singlequote_escape(self, t: _RuleToken) -> None:
        r"\\."
        assert isinstance(t.lexer.string_value, str)
        t.lexer.string_value += typing.cast(str, t.value)[1]

    def t_singlequote_end(self, t: _RuleToken) -> _RuleToken:
        r"'"
        t.value = t.lexer.string_value
        t.type = "ID"
        t.quoted = True
        t.lexer.string_value = None
        t.lexer.pop_state()
        return t

    def t_singlequote_error(self, t: _RuleToken) -> typing.Never:
        value = typing.cast(str, t.value)
        raise JsonPathLexerError(
            f"Error on line {t.lexer.lineno}, col {t.lexpos - t.lexer.latest_newline} "
            f"while lexing singlequoted field: Unexpected character: {value[0]}"
        )

    # Double-quoted strings
    t_doublequote_ignore = ""

    def t_doublequote(self, t: _RuleToken) -> None:
        r'"'
        t.lexer.string_value = ""
        t.lexer.push_state("doublequote")

    def t_doublequote_content(self, t: _RuleToken) -> None:
        r'[^"\\]+'
        assert isinstance(t.lexer.string_value, str)
        t.lexer.string_value += typing.cast(str, t.value)

    def t_doublequote_escape(self, t: _RuleToken) -> None:
        r"\\."
        assert isinstance(t.lexer.string_value, str)
        t.lexer.string_value += typing.cast(str, t.value)[1]

    def t_doublequote_end(self, t: _RuleToken) -> _RuleToken:
        r'"'
        t.value = t.lexer.string_value
        t.type = "ID"
        t.quoted = True
        t.lexer.string_value = None
        t.lexer.pop_state()
        return t

    def t_doublequote_error(self, t: _RuleToken) -> typing.Never:
        value = typing.cast(str, t.value)
        raise JsonPathLexerError(
            f"Error on line {t.lexer.lineno}, col {t.lexpos - t.lexer.latest_newline} "
            f"while lexing doublequoted field: Unexpected character: {value[0]}"
        )

    # Back-quoted "magic" operators
    t_backquote_ignore = ""

    def t_backquote(self, t: _RuleToken) -> None:
        r"`"
        t.lexer.string_value = ""
        t.lexer.push_state("backquote")

    def t_backquote_escape(self, t: _RuleToken) -> None:
        r"\\."
        assert isinstance(t.lexer.string_value, str)
        t.lexer.string_value += typing.cast(str, t.value)[1]

    def t_backquote_content(self, t: _RuleToken) -> None:
        r"[^`\\]+"
        assert isinstance(t.lexer.string_value, str)
        t.lexer.string_value += typing.cast(str, t.value)

    def t_backquote_end(self, t: _RuleToken) -> _RuleToken:
        r"`"
        t.value = t.lexer.string_value
        t.type = "NAMED_OPERATOR"
        t.lexer.string_value = None
        t.lexer.pop_state()
        return t

    def t_backquote_error(self, t: _RuleToken) -> typing.Never:
        value = typing.cast(str, t.value)
        raise JsonPathLexerError(
            f"Error on line {t.lexer.lineno}, col {t.lexpos - t.lexer.latest_newline} "
            f"while lexing backquoted operator: Unexpected character: {value[0]}"
        )

    # Counting lines, handling errors
    def t_newline(self, t: _RuleToken) -> None:
        r"\n"
        t.lexer.lineno += 1
        t.lexer.latest_newline = t.lexpos

    def t_error(self, t: _RuleToken) -> typing.Never:
        value = typing.cast(str, t.value)
        raise JsonPathLexerError(
            "Error on line {}, col {}: Unexpected character: {} ".format(
                t.lexer.lineno, t.lexpos - t.lexer.latest_newline, value[0]
            )
        )


if __name__ == "__main__":
    logging.basicConfig()
    lexer = JsonPathLexer()
    for token in lexer.tokenize(sys.stdin.read()):
        print("%-20s%s" % (token.value, token.type))
