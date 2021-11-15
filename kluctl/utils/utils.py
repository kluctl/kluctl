import hashlib
import logging
import os
import shutil
import sys
import tempfile
from concurrent.futures.thread import ThreadPoolExecutor
from datetime import timedelta
from decimal import Decimal

logger = logging.getLogger(__name__)

def get_tmp_base_dir():
    dir = os.path.join(tempfile.gettempdir(), "kluctl")
    os.makedirs(dir, exist_ok=True)
    return dir

def copytree(src, dst, symlinks=False, ignore=None):
    for item in os.listdir(src):
        s = os.path.join(src, item)
        d = os.path.join(dst, item)
        if os.path.isdir(s):
            shutil.copytree(s, d, symlinks, ignore)
        else:
            shutil.copy2(s, d)

def duration(duration_string):  # example: '5d3h2m1s'
    if isinstance(duration_string, timedelta):
        return duration_string
    duration_string = duration_string.lower()
    total_seconds = Decimal('0')
    prev_num = []
    for character in duration_string:
        if character.isalpha():
            if prev_num:
                num = Decimal(''.join(prev_num))
                if character == 'd':
                    total_seconds += num * 60 * 60 * 24
                elif character == 'h':
                    total_seconds += num * 60 * 60
                elif character == 'm':
                    total_seconds += num * 60
                elif character == 's':
                    total_seconds += num
                prev_num = []
        elif character.isnumeric() or character == '.':
            prev_num.append(character)
    return timedelta(seconds=float(total_seconds))

def parse_bool(s, do_raise=False):
    if s.lower() in ["true", "1", "yes"]:
        return True
    if do_raise and s.lower() not in ["false", "0", "no"]:
        raise ValueError("Invalid boolean value %s" % s)
    return False

def calc_dir_hash(dir):
    h = hashlib.sha256()

    for dirpath, dirnames, filenames in os.walk(dir, topdown=True):
        dirnames.sort()  # ensure iteration order is stable (thanks to topdown, we can do this here at this time)
        filenames.sort()
        rel_dirpath = os.path.relpath(dirpath, dir)
        h.update(rel_dirpath.encode("utf-8"))
        for f in filenames:
            h.update(os.path.join(rel_dirpath, f).encode("utf-8"))
            with open(os.path.join(dirpath, f), mode="rb") as file:
                buf = file.read()
                h.update(buf)

    return h.hexdigest()

class DummyExecutor:
    def __init__(self, *args, **kwargs):
        pass

    def shutdown(self):
        pass

    def __enter__(self):
        return self

    def __exit__(self, exc_type, exc_val, exc_tb):
        pass

    def submit(self, f, *args, **kwargs):
        class Future:
            def __init__(self, result, exc):
                self._result = result
                self._exc = exc
            def exception(self):
                return self._exc
            def result(self):
                if self._exc is not None:
                    raise self._exc
                return self._result
        try:
            result = f(*args, **kwargs)
            return Future(result, None)
        except Exception as e:
            return Future(None, e)

if os.environ.get("KLUCTL_NO_THREADS", "false").lower() in ["1", "true"] or (sys.gettrace() is not None and os.environ.get("KLUCTL_IGNORE_DEBUGGER", "false").lower() not in ["1", "true"]):
    print("Detected a debugger, using DummyExecutor for ThreadPool", file=sys.stderr)
    class MyThreadPoolExecutor(DummyExecutor):
        pass
else:
    class MyThreadPoolExecutor(ThreadPoolExecutor):

        def __init__(self, max_workers=os.cpu_count(), *args, **kwargs) -> None:
            super().__init__(max_workers=max_workers, *args, **kwargs)
