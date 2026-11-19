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

from .. import DatumInContext
from .. import JSONPath

OPERATOR_MAP = {
    "+": operator.add,
    "-": operator.sub,
    "*": operator.mul,
    "/": operator.truediv,
}


class Operation(JSONPath):
    def __init__(self, left, op, right) -> None:
        self.left = left
        self.op_symbol = op
        self.op = OPERATOR_MAP[op]
        self.right = right

    def find(self, datum):
        result = []
        if isinstance(self.left, JSONPath) and isinstance(self.right, JSONPath):
            left = self.left.find(datum)
            right = self.right.find(datum)
            if left and right and len(left) == len(right):
                for left_datum, right_datum in zip(left, right):
                    try:
                        result.append(self.op(left_datum.value, right_datum.value))
                    except TypeError:
                        return []
            else:
                return []
        elif isinstance(self.left, JSONPath):
            left = self.left.find(datum)
            for left_datum in left:
                try:
                    result.append(self.op(left_datum.value, self.right))
                except TypeError:
                    return []
        elif isinstance(self.right, JSONPath):
            right = self.right.find(datum)
            for right_datum in right:
                try:
                    result.append(self.op(self.left, right_datum.value))
                except TypeError:
                    return []
        else:
            try:
                result.append(self.op(self.left, self.right))
            except TypeError:
                return []
        return [DatumInContext.wrap(r) for r in result]

    def __repr__(self):
        return "{}({!r}{}{!r})".format(
            self.__class__.__name__, self.left, self.op_symbol, self.right
        )

    def __str__(self):
        return f"{self.left} {self.op_symbol} {self.right}"

    def __eq__(self, other):
        return (
            isinstance(other, Operation)
            and self.left == other.left
            and self.op_symbol == other.op_symbol
            and self.right == other.right
        )
