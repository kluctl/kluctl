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
from kluctl.utils.gitlab.fast_ls_remote import gitlab_fast_ls_remote
from kluctl.utils.utils import get_tmp_base_dir

logger = logging.getLogger(__name__)

def get_cache_base_dir():
    dir = os.path.join(get_tmp_base_dir(), "git-cache")
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
        if username is None:
            yield None
            return
        # Must handle closing/deletion manually as otherwise git will complain about busy files
        password_script = NamedTemporaryFile("w+t", dir=get_tmp_base_dir(), delete=False)
        password_file = NamedTemporaryFile("w+t", dir=get_tmp_base_dir(), delete=False)

        password_file.write(password)
        password_script.write(f"#!/usr/bin/env sh\ncat {password_file.name}")
        password_file.close()
        password_script.close()
        os.chmod(password_script.name, 0o700)

        g.update_environment(GIT_ASKPASS=password_script.name)
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
            remote_refs = gitlab_fast_ls_remote(url)
            if remote_refs is not None:
                local_refs = git_ls_remote(cache_dir)
                if remote_refs == local_refs:
                    return

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
    ret = gitlab_fast_ls_remote(url, tags=tags)
    if ret is not None:
        return ret

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

@dataclasses.dataclass
class GitUrl:
    schema: str
    host: str
    path: str

def parse_git_url(p, allow_gitlab_fallback=False):
    dummy = "wildcard" * 10
    replaced = p.replace("*", dummy)
    def replace_back(s):
        return s.replace(dummy, "*")
    def trim_git_suffix(s):
        if s.endswith(".git"):
            return s[:-len(".git")]
        return s
    def trim_username(host):
        at_idx = host.find('@')
        if at_idx != -1:
            host = host[at_idx + 1:]
        return host

    if replaced.startswith("http://") or replaced.startswith("https://") or replaced.startswith("ssh://"):
        schema = replaced.split(":", 1)[0]
        url = urlparse(replaced)
        host = url.hostname
        path = url.path[1:]  # trim /
    elif ":" in replaced:
        schema = "ssh"
        s = replaced.split(":", 1)
        host = trim_username(s[0])
        path = s[1]
    elif allow_gitlab_fallback:
        return parse_git_url("https://gitlab.com/%s" % p, False)
    else:
        raise Exception("Invalid git url: %s" % p)
    path = trim_git_suffix(path)
    return GitUrl(schema, replace_back(host), replace_back(path))

def check_git_url_match(a, b, allow_gitlab_fallback):
    a = parse_git_url(a, allow_gitlab_fallback)
    b = parse_git_url(b, allow_gitlab_fallback)
    return fnmatch.fnmatch(a.host, b.host) and fnmatch.fnmatch(a.path, b.path)

def guess_git_web_url(git_url):
    a = parse_git_url(git_url)
    if a.host.find("gitlab") != -1:
        return f"https://{a.host}/{a.path}"
    if a.host.find("bitbucket") != -1:
        if a.schema in ("http", "https"):
            s = a.path.split("/")
            if len(s) < 3:
                return git_url

            group = s[1]
            project = s[2]
            return f"{a.schema}://{a.host}/scm/{group}/{project}.git"
        elif a.schema == "ssh":
            s = a.path.split("/")
            if len(s) < 2:
                return git_url
            group = s[0]
            project = s[1]
            host = a.host.split(":", 1)[0]
            return f"https://{host}/scm/{group}/{project}"
    return git_url
