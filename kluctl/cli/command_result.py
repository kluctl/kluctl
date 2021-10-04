from deepdiff.helper import NotPresent

from kluctl.diff.k8s_diff import unified_diff_object
from kluctl.utils.k8s_object_utils import get_long_object_name, get_long_object_name_from_ref, get_object_ref
from kluctl.utils.pretty_table import pretty_table


def format_command_result_tables(new_objects, changed_objects, orphan_objects):
    result = ''

    if new_objects:
        result += "New objects:\n"
        for x in new_objects:
            result += "  %s\n" % get_long_object_name(x, include_api_version=False)

    if changed_objects:
        result += "Changed objects:\n"
        for x in changed_objects:
            result += "  %s\n" % get_long_object_name(x["new_object"], include_api_version=False)

        result += "\n"
        for x in changed_objects:
            object = x["new_object"]
            changes = x["changes"]
            result += "%s\n" % pretty_changes(get_object_ref(object), changes)

    if orphan_objects:
        result += "Orphan objects:\n"
        for ref in orphan_objects:
            result += "  %s\n" % get_long_object_name_from_ref(ref)

    return result

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
