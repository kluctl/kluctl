import logging
import os
import shutil
import subprocess
import sys
import tempfile
import threading
from concurrent.futures.thread import ThreadPoolExecutor
from datetime import timedelta
from decimal import Decimal
from io import BytesIO

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

def is_iterable(obj):
    try:
        iter(obj)
    except Exception:
        return False
    else:
        return True

def copy_primitive_value(v):
    if isinstance(v, dict):
        return copy_dict(v)
    if isinstance(v, str) or isinstance(v, bytes):
        return v
    if is_iterable(v):
        return [copy_primitive_value(x) for x in v]
    return v

def copy_dict(a):
    ret = {}
    for k, v in a.items():
        ret[k] = copy_primitive_value(v)
    return ret

def merge_dict(a, b, clone=True):
    if clone:
        a = copy_dict(a)
    if a is None:
        a = {}
    if b is None:
        b = {}
    for key in b:
        if key in a:
            if isinstance(a[key], dict) and isinstance(b[key], dict):
                merge_dict(a[key], b[key], clone=False)
            else:
                a[key] = b[key]
        else:
            a[key] = b[key]
    return a

def duration(duration_string):  # example: '5d3h2m1s'
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

def set_default_value(d, n, default):
    if n not in d or d[n] is None:
        d[n] = default

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
            def result(self):
                return f(*args, **kwargs)
        return Future()

if os.environ.get("KLUCTL_NO_THREADS", "false").lower() in ["1", "true"] or (sys.gettrace() is not None and os.environ.get("KLUCTL_IGNORE_DEBUGGER", "false").lower() not in ["1", "true"]):
    print("Detected a debugger, using DummyExecutor for ThreadPool", file=sys.stderr)
    class MyThreadPoolExecutor(DummyExecutor):
        pass
else:
    class MyThreadPoolExecutor(ThreadPoolExecutor):

        def __init__(self, max_workers=os.cpu_count(), *args, **kwargs) -> None:
            super().__init__(max_workers=max_workers, *args, **kwargs)
