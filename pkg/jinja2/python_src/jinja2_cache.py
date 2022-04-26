import errno
import fnmatch
import os
import stat
import tempfile
import typing
from datetime import datetime
from types import CodeType

from jinja2 import BytecodeCache
from jinja2.bccache import Bucket

def get_tmp_base_dir():
    dir = os.path.join(tempfile.gettempdir(), "kluctl-workdir")
    os.makedirs(dir, exist_ok=True)
    return dir

"""
A bytecode cache that is able to share bytecode between different directories. It does this by removing filenames
from dumped code and then later re-adding them after loading.

This cache is also thread and multi-process safe.
"""
class KluctlBytecodeCache(BytecodeCache):
    def __init__(self, max_cache_files):
        self.cache_dir = self._get_cache_dir()
        self.max_cache_files = max_cache_files
        self.did_touch = set()
        self.clear_old_entries()

    def _get_cache_dir(self) -> str:
        def _unsafe_dir() -> "te.NoReturn":
            raise RuntimeError(
                "Cannot determine safe temp directory.  You "
                "need to explicitly provide one."
            )

        tmpdir = os.path.join(get_tmp_base_dir(), "jinja2-cache")

        # On windows the temporary directory is used specific unless
        # explicitly forced otherwise.  We can just use that.
        if os.name == "nt":
            os.makedirs(tmpdir, exist_ok=True)
            return tmpdir
        if not hasattr(os, "getuid"):
            _unsafe_dir()

        tmpdir = os.path.join(tmpdir, str(os.getuid()))

        try:
            os.makedirs(tmpdir, exist_ok=True, mode=stat.S_IRWXU)
        except OSError as e:
            if e.errno != errno.EEXIST:
                raise
        try:
            os.chmod(tmpdir, stat.S_IRWXU)
            actual_dir_stat = os.lstat(tmpdir)
            if (
                actual_dir_stat.st_uid != os.getuid()
                or not stat.S_ISDIR(actual_dir_stat.st_mode)
                or stat.S_IMODE(actual_dir_stat.st_mode) != stat.S_IRWXU
            ):
                _unsafe_dir()
        except OSError as e:
            if e.errno != errno.EEXIST:
                raise

        actual_dir_stat = os.lstat(tmpdir)
        if (
            actual_dir_stat.st_uid != os.getuid()
            or not stat.S_ISDIR(actual_dir_stat.st_mode)
            or stat.S_IMODE(actual_dir_stat.st_mode) != stat.S_IRWXU
        ):
            _unsafe_dir()

        return tmpdir

    def get_cache_key(self, name: str, filename: typing.Optional[typing.Union[str]] = None):
        return (name, filename)

    def _get_cache_filename(self, bucket: Bucket) -> str:
        return os.path.join(self.cache_dir, bucket.checksum[:2], bucket.checksum) + ".cache"

    def touch_loaded_marker(self, bucket):
        filename = self._get_cache_filename(bucket) + ".loaded"
        if filename in self.did_touch:
            return
        try:
            with open(filename + ".tmp", mode="wt") as f:
                f.write(str(datetime.utcnow()))
            os.rename(filename + ".tmp", filename)
        except:
            # Failure here it "ok" and is mostly happening on Windows here (permission denied for opened files...ugh..)
            pass
        self.did_touch.add(filename)

    def replace_code_filenames(self, code, old, new):
        co_filename = code.co_filename
        co_consts = []
        if co_filename == old:
            co_filename = new

        for c in code.co_consts:
            if isinstance(c, CodeType):
                co_consts.append(self.replace_code_filenames(c, old, new))
            else:
                co_consts.append(c)

        return code.replace(co_filename=co_filename, co_consts=tuple(co_consts))

    def load_bytecode(self, bucket: Bucket) -> None:
        filename = self._get_cache_filename(bucket)

        if os.path.exists(filename):
            for i in range(4):
                try:
                    with open(filename, "rb") as f:
                        bucket.load_bytecode(f)
                        if bucket.code is not None:
                            bucket.code = self.replace_code_filenames(bucket.code, "<dummy>", bucket.key[1])
                except PermissionError:
                    # Retry. Windows is still failing from time to time...
                    continue
            self.touch_loaded_marker(bucket)

    def dump_bytecode(self, bucket: Bucket) -> None:
        # thread/multi-process safe version of super().dump_bytecode()

        try:
            filename = self._get_cache_filename(bucket)
            os.makedirs(os.path.dirname(filename), exist_ok=True)

            with open(filename + ".tmp", "wb") as f:
                orig_code = bucket.code
                bucket.code = self.replace_code_filenames(bucket.code, bucket.key[1], "<dummy>")
                bucket.write_bytecode(f)
                bucket.code = orig_code
            os.rename(filename + ".tmp", filename)
            self.touch_loaded_marker(bucket)
        except:
            # Failure here it "ok" and is mostly happening on Windows here (permission denied for opened files...ugh..)
            pass

    def clear_old_entries(self):
        files = []
        for d in os.listdir(self.cache_dir):
            path = os.path.join(self.cache_dir, d)
            if not os.path.isdir(path):
                continue
            for n in os.listdir(path):
                if not fnmatch.fnmatch(n, "*.cache"):
                    continue
                files.append(os.path.join(path, n))

        if len(files) <= self.max_cache_files:
            return
        times = {}
        for f in files:
            try:
                with open(f + ".loaded", mode="rt") as f2:
                    times[f] = datetime.fromisoformat(f2.read())
            except:
                times[f] = datetime.min
        files.sort(key=lambda f: times[f], reverse=True)
        for f in files[self.max_cache_files:]:
            try:
                os.remove(f)
                os.remove(f + ".loaded")
            except:
                pass
