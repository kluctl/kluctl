import hashlib
import shutil
import threading

from kluctl.command_error import CommandError

def get_external_tool_path(name):
    path = shutil.which(name)
    if path is None:
        raise CommandError("%s was not found. Is it installed and in your PATH?" % name)
    return path

external_tool_hashes = {}
external_tool_hashes_lock = threading.Lock()
def get_external_tool_hash(name):
    global external_tool_hashes
    global external_tool_hashes_lock
    with external_tool_hashes_lock:
        if name not in external_tool_hashes:
            external_tool_hashes[name] = calc_external_tool_hash(name)
        return external_tool_hashes[name]

def calc_external_tool_hash(name):
    with open(get_external_tool_path(name), "rb") as f:
        return hashlib.sha256(f.read()).hexdigest()
