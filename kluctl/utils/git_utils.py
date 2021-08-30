import dataclasses
import fnmatch
import hashlib
import logging
import os
import re
from contextlib import contextmanager
from tempfile import NamedTemporaryFile
from urllib.parse import urlparse

import filelock
from git import Git

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

class GitCredentialsStore:
    def get_credentials(self, host):
        return None, None

class GitCredentialStoreEnv(GitCredentialsStore):
    def get_credentials(self, host):
        for idx, s in parse_env_config_sets("KLUCTL_GIT").items():
            if s.get("HOST") == host:
                return s.get("USERNAME"), s.get("PASSWORD")
        return None, None

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
    return f"{u.schema}://{username}@{u.host}/{u.path}"

@contextmanager
def build_git_object(url, working_dir):
    username = None
    password = None
    try:
        u = parse_git_url(url)
        username, password = get_git_credentials_store().get_credentials(u.host)
    except:
        pass

    g = Git(working_dir)
    g.update_environment(
        GIT_SSH_COMMAND="ssh -o 'ControlMaster=auto' -o 'ControlPath=/tmp/agent_ralf_control_master-%r@%h-%p' -o 'ControlPersist=5m'"
    )

    if username is not None:
        url = add_username_to_url(url, username)

    @contextmanager
    def create_password_files():
        # Must handle closing/deletion manually as otherwise git will complain about busy files
        password_script = NamedTemporaryFile("w+t", dir=get_tmp_base_dir(), delete=False)
        password_file = NamedTemporaryFile("w+t", dir=get_tmp_base_dir(), delete=False)

        if username is not None:
            password_file.write(password)
            password_script.write(f"#!/usr/bin/env sh\ncat {password_file.name}")
        else:
            password_script.write(NO_CREDENTIALS_PROMPT % url)

        password_file.close()
        password_script.close()
        os.chmod(password_script.name, 0o700)

        g.update_environment(GIT_ASKPASS=password_script.name, GIT_TERMINAL_PROMPT="0")
        try:
            yield None
        finally:
            try:
                os.unlink(password_file.name)
            except:
                pass
            try:
                os.unlink(password_script.name)
            except:
                pass

    with create_password_files():
        yield g, url

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
            g.fetch("origin", "-f")

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
    def trim_git_suffix(s):
        if s.endswith(".git"):
            return s[:-len(".git")]
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
        path = trim_git_suffix(url.path)
        port = normalize_port(url.scheme, url.port)
        return GitUrl(url.scheme, url.hostname, port, path, url.username)

    pattern = re.compile("(.+@)?([\w\d\.]+):(,*)")
    m = pattern.match(p)
    if not m:
        raise Exception("Invalid git url %s" % p)

    username = m.group(1)
    if username is not None:
        username = username[:-1]
    host = m.group(2)
    path = m.group(3)
    return GitUrl("ssh", host, 22, path, username)

def check_git_url_match(a, b):
    a = parse_git_url(a)
    b = parse_git_url(b)
    return a == b
