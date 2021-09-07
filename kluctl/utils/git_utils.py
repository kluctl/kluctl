import dataclasses
import hashlib
import logging
import os
import re
import shutil
import sys
from contextlib import contextmanager
from tempfile import NamedTemporaryFile
from urllib.parse import urlparse

import filelock
from git import Git, GitCommandError

from kluctl.utils.env_config_sets import parse_env_config_sets
from kluctl.utils.utils import get_tmp_base_dir

logger = logging.getLogger(__name__)

NO_CREDENTIALS_PROMPT = """#!/usr/bin/env sh
echo >&2
echo 'Interactive password prompts for git are disabled when running kluctl.' >&2
echo 'Please ensure credentials for %s are somehow setup.' >&2
echo 'This can for example be achieved by running a manual git clone operation' >&2
echo 'with a configured credential helper beforehand.' >&2
echo >&2
exit 1
"""

def get_cache_base_dir():
    dir = os.path.join(get_tmp_base_dir(), "git-cache")
    logger.debug("cache base dir: %s" % dir)
    return dir

@dataclasses.dataclass()
class GitCredentials:
    host: str
    username: str
    password: str
    ssh_key: str
    path_prefix: str

class GitCredentialsStore:
    def get_credentials(self, host, path):
        return None, None, None

    def check_credentials(self, c, test_host, test_path):
        if c.host != test_host:
            return False
        if not test_path.startswith(c.path_prefix or ""):
            return False
        return True

    def find_matching_credentials(self, credentials, test_host, test_path):
        c = [x for x in credentials if self.check_credentials(x, test_host, test_path)]
        if not c:
            return None
        c.sort(key=lambda x: len(x.path_prefix))
        # return credentials with longest matching path
        return c[-1]

class GitCredentialStoreEnv(GitCredentialsStore):
    def get_credentials(self, host, path):
        credentials = []
        for idx, s in parse_env_config_sets("KLUCTL_GIT").items():
            credentials.append(GitCredentials(host=s.get("HOST"),
                                              username=s.get("USERNAME"),
                                              password=s.get("PASSWORD"),
                                              ssh_key=s.get("SSH_KEY"),
                                              path_prefix=s.get("PATH_PREFIX", "")))
        c = self.find_matching_credentials(credentials, host, path)
        if c is not None and c.ssh_key is not None:
            path = os.path.expanduser(c.ssh_key)
            with open(path, "rt") as f:
                c.ssh_key = f.read()
        return c

credentials_store = GitCredentialStoreEnv()

def set_git_credentials_store(store):
    global credentials_store
    credentials_store = store

def get_git_credentials_store():
    return credentials_store

def build_remote_name(url):
    remote_name = os.path.basename(url)
    if remote_name.endswith(".git"):
        remote_name = remote_name[:-len(".git")]
    remote_name += "-" + hashlib.sha256(url.encode()).hexdigest()[:6]
    return remote_name

def add_username_to_url(url, username):
    if username is None:
        return url
    u = parse_git_url(url)
    return f"{u.schema}://{username}@{u.host}:{u.port}/{u.path}"

@contextmanager
def create_password_files(g, ssh_command, url, credentials):
    # Must handle closing/deletion manually as otherwise git will complain about busy files
    password_script = NamedTemporaryFile("w+t", dir=get_tmp_base_dir(), delete=False)
    password_file = NamedTemporaryFile("w+t", dir=get_tmp_base_dir(), delete=False)
    ssh_key_file = NamedTemporaryFile("w+t", dir=get_tmp_base_dir(), delete=False)

    if credentials is not None and credentials.password is not None:
        password_file.write(credentials.password)
        password_script.write(f"#!/usr/bin/env sh\ncat {password_file.name}")
    else:
        password_script.write(NO_CREDENTIALS_PROMPT % url)

    if credentials is not None and credentials.ssh_key is not None:
        ssh_key_file.write(credentials.ssh_key)
        ssh_command += " -i '%s'" % ssh_key_file.name

    for x in [password_script, password_file, ssh_key_file]:
        x.close()
    os.chmod(password_script.name, 0o700)

    g.update_environment(GIT_ASKPASS=password_script.name, GIT_TERMINAL_PROMPT="0")
    try:
        yield ssh_command
    finally:
        for x in [password_script, password_file, ssh_key_file]:
            try:
                os.unlink(x.name)
            except Exception:
                pass

@contextmanager
def build_git_object(url, working_dir):
    u = parse_git_url(url)
    credentials = get_git_credentials_store().get_credentials(u.host, u.path)
    if credentials is not None and u.username is not None and u.username != credentials.username:
        raise Exception("username from url does not match username from credentials store")

    class MyGit(Git):
        def execute(self, command, **kwargs):
            kwargs2 = kwargs.copy()
            if "KLUCTL_GIT_TIMEOUT" in os.environ:
                kwargs2["kill_after_timeout"] = int(os.environ["KLUCTL_GIT_TIMEOUT"])
            return super().execute(command, **kwargs2)

    g = MyGit(working_dir)

    ssh_command = os.environ.get("GIT_SSH", "ssh")
    ssh_command += " -o 'StrictHostKeyChecking=no'"

    if sys.platform != "win32":
        ssh_command += " -o 'ControlMaster=auto'"
        ssh_command += " -o 'ControlPath=/tmp/kluctl_control_master-%r@%h-%p'"
        ssh_command += " -o 'ControlPersist=5m'"

    if credentials is not None and credentials.username is not None:
        url = add_username_to_url(url, credentials.username)

    with create_password_files(g, ssh_command, url, credentials) as ssh_command:
        g.update_environment(
            GIT_SSH_COMMAND=ssh_command
        )
        try:
            yield g, url
        finally:
            pass

@contextmanager
def with_git_cache(url, do_lock=True):
    remote_name = build_remote_name(url)
    cache_dir = os.path.join(get_cache_base_dir(), remote_name)
    if not os.path.exists(cache_dir):
        os.makedirs(cache_dir, exist_ok=True)
    lock_file = os.path.join(cache_dir, ".cache.lock")
    init_marker = os.path.join(cache_dir, ".cache.init")

    lock = filelock.FileLock(lock_file)
    if do_lock:
        lock.acquire()

    try:
        if not os.path.exists(init_marker):
            logger.info(f"Creating bare repo at {cache_dir}")
            with build_git_object(url, cache_dir) as (g, url):
                g.init("--bare")
                g.remote("add", "origin", url)
                g.config("remote.origin.fetch", "refs/heads/*:refs/heads/*")
            with open(init_marker, "w"):
                # only touch it
                pass
        yield cache_dir
    finally:
        if do_lock:
            lock.release()

def update_git_cache(url, do_lock=True):
    with with_git_cache(url, do_lock=do_lock) as cache_dir:
        with build_git_object(url, cache_dir) as (g, url):
            logger.info(f"Fetching into cache: url='{url}'")
            try:
                g.fetch("origin", "-f")
            except GitCommandError as e:
                if "did not complete in " in e.stderr:
                    logger.info("Git command timed out, deleting cache (%s) to ensure that we don't get into an "
                                "inconsistent state" % cache_dir)
                    try:
                        shutil.rmtree(cache_dir)
                    except:
                        pass
                raise

def clone_project(url, ref, target_dir, git_cache_up_to_date=None):
    logger.info(f"Cloning git project: url='{url}', ref='{ref}'")

    with with_git_cache(url) as cache_dir:
        if git_cache_up_to_date is None or url not in git_cache_up_to_date:
            update_git_cache(url, do_lock=False)
        args = ["file://%s" % cache_dir, "--single-branch", target_dir]
        if ref is not None:
            args += ["--branch", ref]
        Git().clone(*args)

def get_git_commit(path):
    g = Git(path)
    commit = g.rev_parse("HEAD", stdout_as_string=True)
    return commit.strip()

def get_git_ref(path):
    g = Git(path)
    branch = g.rev_parse("--abbrev-ref", "HEAD", stdout_as_string=True).strip()
    if branch != "HEAD":
        return branch
    tag = g.describe("--tags", stdout_as_string=True).strip()
    return tag

def git_ls_remote(url, tags=False):
    args = []
    if tags:
        args.append("--tags")
    with build_git_object(url, None) as (g, url):
        args.append(url)
        txt = g.ls_remote("-q", *args)
    lines = txt.splitlines()
    ret = {}
    for l in lines:
        x = l.split()
        ret[x[1]] = x[0]
    return ret

def filter_remote_refs(refs, pattern, trim):
    pattern = re.compile(r"refs/heads/%s" % pattern)
    matching_refs = {}
    for r, commit in refs.items():
        if pattern.match(r):
            r2 = r
            if trim:
                r2 = r[len("refs/heads/"):]
            matching_refs[r2] = commit
    return matching_refs

@dataclasses.dataclass(eq=True)
class GitUrl:
    schema: str
    host: str
    port: int
    path: str
    username: str

def parse_git_url(p):
    def trim_path(s):
        if s.startswith("/"):
            s = s[1:]
        if s.endswith(".git"):
            s = s[:-len(".git")]
        return s
    def normalize_port(schema, port):
        if port is not None:
            return port
        if schema == "http":
            return 80
        if schema == "https":
            return 443
        if schema == "ssh":
            return 22
        raise Exception("Unknown schema %s" % schema)

    schema_pattern = re.compile("^([a-z]*)://.*")
    m = schema_pattern.match(p)
    if m:
        url = urlparse(p)
        path = trim_path(url.path)
        port = normalize_port(url.scheme, url.port)
        return GitUrl(url.scheme, url.hostname, port, path, url.username)

    pattern = re.compile("(.+@)?([\w\d\.]+):(.*)")
    m = pattern.match(p)
    if not m:
        raise Exception("Invalid git url %s" % p)

    username = m.group(1)
    if username is not None:
        username = username[:-1]
    host = m.group(2)
    path = trim_path(m.group(3))
    return GitUrl("ssh", host, 22, path, username)

def check_git_url_match(a, b):
    a = parse_git_url(a)
    b = parse_git_url(b)
    return a == b
