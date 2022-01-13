import dataclasses
import os

import click
from deepdiff.helper import NotPresent

from kluctl.cli.utils import build_seen_images
from kluctl.deployment.deployment_collection import CommandResult
from kluctl.diff.k8s_diff import unified_diff_object, changes_to_yaml
from kluctl.utils.exceptions import CommandError
from kluctl.utils.k8s_object_utils import get_long_object_name, get_long_object_name_from_ref, get_object_ref
from kluctl.utils.pretty_table import pretty_table
from kluctl.utils.yaml_utils import yaml_dump, yaml_dump_all


def format_command_result_text(command_result: CommandResult):
    result = ''

    if command_result.warnings:
        result += "\nWarnings:\n"
        result += pretty_errors(command_result.warnings)

    if command_result.new_objects:
        result += "\nNew objects:\n"
        for x in command_result.new_objects:
            result += "  %s\n" % get_long_object_name(x, include_api_version=False)

    if command_result.changed_objects:
        result += "\nChanged objects:\n"
        for x in command_result.changed_objects:
            result += "  %s\n" % get_long_object_name(x["new_object"], include_api_version=False)

        result += "\n"
        for x in command_result.changed_objects:
            object = x["new_object"]
            changes = x["changes"]
            result += "%s" % pretty_changes(get_object_ref(object), changes)

    if command_result.deleted_objects:
        result += "\nDeleted objects:\n"
        for x in command_result.deleted_objects:
            result += "  %s\n" % get_long_object_name_from_ref(x)

    if command_result.hook_objects:
        result += "\nApplied hooks:\n"
        for x in command_result.hook_objects:
            result += "  %s\n" % get_long_object_name(x)

    if command_result.orphan_objects:
        result += "\nOrphan objects:\n"
        for ref in command_result.orphan_objects:
            result += "  %s\n" % get_long_object_name_from_ref(ref)

    if command_result.errors:
        result += "\nErrors:\n"
        result += pretty_errors(command_result.errors)

    return result

def pretty_errors(errors):
    table = [("Object", "Message")]
    for e in errors:
        table.append((get_long_object_name_from_ref(e.ref), e.message))
    return pretty_table(table, [60])

def pretty_changes(ref, changes):
    ret = 'Diff for object %s\n' % get_long_object_name_from_ref(ref)

    table = [('Path', 'Diff')]
    for c in changes:
        if "unified_diff" in c:
            diff = c["unified_diff"]
        else:
            diff = unified_diff_object(c.get("old_value", NotPresent()), c.get("new_value", NotPresent()))
        table.append((c["path"], diff))

    ret += pretty_table(table, [60])

    return ret

def format_command_result_yaml(c, command_result: CommandResult):
    result = {
        "diff": changes_to_yaml(command_result.new_objects, command_result.changed_objects),
        "deleted_objects": command_result.deleted_objects,
        "applied_hooks": command_result.hook_objects,
        "orphan_objects": [{"ref": dataclasses.asdict(ref)} for ref in command_result.orphan_objects],
        "errors": [dataclasses.asdict(x) for x in command_result.errors],
        "warnings": [dataclasses.asdict(x) for x in command_result.warnings],
        "images": build_seen_images(c, True),
    }
    return yaml_dump(result)

def format_command_result(c, command_result, format):
    if format == "text":
        return format_command_result_text(command_result)
    elif format == "yaml":
        return format_command_result_yaml(c, command_result)
    else:
        raise CommandError(f"Invalid format: {format}")

def format_validate_result(result, format):
    if format == "text":
        str = ""
        if result.warnings:
            str += "Validation Warnings:\n"
            for item in result.warnings:
                str += "  %s: reason=%s, message=%s\n" % (get_long_object_name_from_ref(item.ref), item.reason, item.message)
        if result.errors:
            if str:
                str += "\n"
            str += "Validation Errors:\n"
            for item in result.errors:
                str += "  %s: reason=%s, message=%s\n" % (get_long_object_name_from_ref(item.ref), item.reason, item.message)
        if result.results:
            if str:
                str += "\n"
            str += "Results:\n"
            for item in result.results:
                str += "  %s: reason=%s, message=%s\n" % (get_long_object_name_from_ref(item.ref), item.reason, item.message)
        return str
    if format == "yaml":
        y = yaml_dump(dataclasses.asdict(result))
        return y
    else:
        raise CommandError(f"Invalid format: {format}")


def output_command_result(output, c, command_result):
    if not output:
        output = ["text"]
    for o in output:
        s = o.split("=", 1)
        format = s[0]
        path = None
        if len(s) > 1:
            path = s[1]
        s = format_command_result(c, command_result, format)
        output_result(path, s)


def output_validate_result(output, result):
    if not output:
        output = ["text"]
    for o in output:
        s = o.split("=", 1)
        format = s[0]
        path = None
        if len(s) > 1:
            path = s[1]
        s = format_validate_result(result, format)
        output_result(path, s)


def output_yaml_result(output, result, all=False):
    output = output or [None]
    if all:
        s = yaml_dump_all(result)
    else:
        s = yaml_dump(result)
    for o in output:
        output_result(o, s)


def output_result(output_file, result):
    path = None
    if output_file and output_file != "-":
        path = os.path.expanduser(output_file)
    if path is None:
        click.echo(result)
    else:
        with open(path, "wt") as f:
            f.write(result)
