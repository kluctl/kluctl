#!/usr/bin/python

# Copyright 2012 Felix Richter <wtfpl@syntax-fehler.de>
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import argparse
import glob
import json
import sys

from jsonpath_ng import parse


def main():
    parser = argparse.ArgumentParser(
        description="Search JSON files (or stdin) according to a JSONPath expression.",
        formatter_class=argparse.RawTextHelpFormatter,
        epilog="""
        Quick JSONPath reference
        (see more at https://github.com/jsonpath-ng/jsonpath-ng/)

        atomics:
            $              - root object
            `this`         - current object

        operators:
            path1.path2    - same as xpath /
            path1|path2    - union
            path1..path2   - somewhere in between

        fields:
            fieldname       - field with name
            *               - any field
            [_start_?:_end_?] - array slice
            [*]             - any array index
    """,
    )

    parser.add_argument("expression", help="A JSONPath expression.")
    parser.add_argument(
        "files",
        metavar="file",
        nargs="*",
        help="Files to search (if none, searches stdin)",
    )

    args = parser.parse_args(sys.argv[1:])

    expr = parse(args.expression)
    glob_patterns = args.files

    if len(glob_patterns) == 0:
        # stdin mode
        _print_matches(_find_matches_for_file(expr, sys.stdin))
    else:
        # file paths mode
        for pattern in glob_patterns:
            for filename in glob.glob(pattern):
                with open(filename) as f:
                    _print_matches(_find_matches_for_file(expr, f))


def _find_matches_for_file(expr, f):
    return expr.find(json.load(f))


def _print_matches(matches):
    for match in matches:
        print(json.dumps(match.value))


if __name__ == "__main__":
    main()
