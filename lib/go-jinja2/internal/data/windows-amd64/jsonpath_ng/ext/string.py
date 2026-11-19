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

import re

from .. import DatumInContext
from .. import This


class DefintionInvalid(Exception):
    pass


# The `sub` definition is matched in two steps to avoid quadratic backtracking.
# A single `sub\(/(.*)/,\s+(.*)\)` rescans the rest of the line
# for every `/,` candidate when the final `)` is missing.
# Matching the text up to the last `)` first makes every candidate viable.
_SUB_BODY = re.compile(r"sub\(/(.*)\)")
_SUB_PARTS = re.compile(r"(.*)/,\s+(.*)")


class Sub(This):
    """Regex substituor

    Concrete syntax is '`sub(/regex/, repl)`'
    """

    def __init__(self, method=None) -> None:
        m = _SUB_BODY.match(method)
        if m is not None:
            m = _SUB_PARTS.fullmatch(m.group(1))
        if m is None:
            raise DefintionInvalid("%s is not valid" % method)
        self.expr = m.group(1).strip()
        self.repl = m.group(2).strip()
        self.regex = re.compile(self.expr)
        self.method = method

    def find(self, datum):
        datum = DatumInContext.wrap(datum)
        value = self.regex.sub(self.repl, datum.value)
        if value == datum.value:
            return []
        else:
            return [DatumInContext.wrap(value)]

    def __eq__(self, other):
        return isinstance(other, Sub) and self.method == other.method

    def __repr__(self):
        return f"{self.__class__.__name__}({self.method!r})"

    def __str__(self):
        return f"`sub(/{self.expr}/, {self.repl})`"


_SPLIT = re.compile(
    r"split\s*+\("
    r"(?:\s*+|\s*(?=\s(?!\s)))"
    r"(?P<quote>['\"]?)((?:\S|[^\S\n]++)+)(?P=quote)\s*+,\s*+"
    r"(-?\d++|\*)\s*+,\s*+"
    r"(-?\d++)\s*+\)"
)


class Split(This):
    """String splitter

    Concrete syntax is '`split(chars, segment, max_split)`'
    `chars` can optionally be surrounded by quotes,
    to specify things like commas or spaces
    `segment` can be `*` to select all
    `max_split` can be negative, to indicate no limit
    """

    def __init__(self, method=None) -> None:
        m = _SPLIT.match(method)
        if m is None:
            raise DefintionInvalid("%s is not valid" % method)
        self.chars = m.group(2)
        self.segment = m.group(3)
        self.max_split = int(m.group(4))
        self.method = method

    def find(self, datum):
        datum = DatumInContext.wrap(datum)
        try:
            if self.segment == "*":
                value = datum.value.split(self.chars, self.max_split)
            else:
                value = datum.value.split(self.chars, self.max_split)[int(self.segment)]
        except Exception:
            return []
        return [DatumInContext.wrap(value)]

    def __eq__(self, other):
        return isinstance(other, Split) and self.method == other.method

    def __repr__(self):
        return f"{self.__class__.__name__}({self.method!r})"

    def __str__(self):
        return "`%s`" % self.method


_STR = re.compile(r"str\(\)")


class Str(This):
    """String converter

    Concrete syntax is '`str()`'
    """

    def __init__(self, method=None) -> None:
        m = _STR.match(method)
        if m is None:
            raise DefintionInvalid("%s is not valid" % method)
        self.method = method

    def find(self, datum):
        datum = DatumInContext.wrap(datum)
        value = str(datum.value)
        return [DatumInContext.wrap(value)]

    def __eq__(self, other):
        return isinstance(other, Str) and self.method == other.method

    def __repr__(self):
        return f"{self.__class__.__name__}({self.method!r})"

    def __str__(self):
        return "`str()`"
