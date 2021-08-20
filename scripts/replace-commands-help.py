import dataclasses
import logging
import os
import re
import subprocess

logger = logging.getLogger(__name__)
logging.basicConfig(level=logging.INFO)

commands_md_path = os.path.join(os.path.dirname(__file__), "../docs/commands.md")

@dataclasses.dataclass
class Section:
    start: int
    end: int
    command: str
    section: str
    code: bool

def find_next_section(lines, start):
    begin_pattern = re.compile(r"<!-- BEGIN SECTION \"([^\"]*)\" \"([^\"]*)\" (true|false) -->")
    end_pattern = re.compile(r"<!-- END SECTION -->")

    for i in range(start, len(lines)):
        m = begin_pattern.match(lines[i])
        if not m:
            continue

        command = m.group(1)
        section = m.group(2)
        code = m.group(3) == "true"

        for j in range(i + 1, len(lines)):
            m = end_pattern.match(lines[j])
            if not m:
                continue

            return Section(start=i, end=j, command=command, section=section, code=code)

    return None

def count_indent(str):
    indent = 0
    for i in range(len(str)):
        if str[i] != " ":
            break
        indent += 1
    return indent

def get_help_section(command, section):
    logger.info("Getting section '%s' from command '%s" % (section, command))

    kluctl_path = os.path.join(os.path.dirname(__file__), "..", "cli.py")
    args = [kluctl_path]
    if command:
        args += [command]
    args += ["--help"]

    r = subprocess.run(args, capture_output=True, text=True, check=True)
    lines = r.stdout.splitlines()
    section_start = None
    for i in range(len(lines)):
        indent = count_indent(lines[i])
        if lines[i][indent:].startswith("%s:" % section):
            section_start = i
            break
    if section_start is None:
        raise Exception("Section %s not found in command %s" % (section, command))

    ret = [lines[section_start] + "\n"]
    section_indent = count_indent(lines[section_start])
    for i in range(section_start + 1, len(lines)):
        indent = count_indent(lines[i])
        if lines[i] != "" and indent <= section_indent:
            break
        ret.append(lines[i] + "\n")
    return ret


with open(commands_md_path) as f:
    lines = f.readlines()

new_lines = []
pos = 0
while True:
    s = find_next_section(lines, pos)
    if s is None:
        new_lines += lines[pos:]
        break

    new_lines += lines[pos:s.start + 1]

    s2 = get_help_section(s.command, s.section)

    if s.code:
        new_lines += ["```\n"]
    new_lines += s2
    if s.code:
        new_lines += ["```\n"]

    new_lines += [lines[s.end]]
    pos = s.end + 1

if lines != new_lines:
    with open(commands_md_path, mode="w") as f:
        f.writelines(new_lines)
