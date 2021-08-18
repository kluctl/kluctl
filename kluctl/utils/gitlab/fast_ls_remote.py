import hashlib
import logging
import os
from datetime import datetime, timedelta

from kluctl.utils.gitlab.gitlab_util import extract_gitlab_group_and_project, build_gitlab_project_id, get_gitlab_api, \
    is_gitlab_project
from kluctl.utils.utils import get_tmp_base_dir

logger = logging.getLogger(__name__)

def _is_backlisted(url):
    hash = hashlib.sha256(url.encode("utf-8")).hexdigest()
    path = os.path.join(get_tmp_base_dir(), "gitlab-blacklist", hash)
    try:
        with open(path) as f:
            s = f.read()
            t = datetime.fromisoformat(s)
            return t >= datetime.utcnow() - timedelta(minutes=5)
    except:
        return False

def _set_blacklisted(url):
    hash = hashlib.sha256(url.encode("utf-8")).hexdigest()
    path = os.path.join(get_tmp_base_dir(), "gitlab-blacklist")
    os.makedirs(path, exist_ok=True)
    path = os.path.join(path, hash)
    try:
        with open(path, mode="wt") as f:
            f.write(str(datetime.utcnow()))
    except:
        pass

def gitlab_fast_ls_remote(url, tags=False):
    if not is_gitlab_project(url):
        return None

    if _is_backlisted(url):
        return None

    try:
        return _gitlab_fast_ls_remote(url, tags)
    except Exception as e:
        logger.warning("Exception while trying fast_get_git_refs_gitlab", exc_info=e)
        _set_blacklisted(url)
        return None

def _gitlab_fast_ls_remote(url, tags):
    gl = get_gitlab_api(require_auth=True)
    if gl is None:
        return None

    result = {}
    group, project = extract_gitlab_group_and_project(url)
    base_path = "/projects/%s/repository" % build_gitlab_project_id(group, project)
    path = "%s/branches" % base_path
    r = gl.http_get(path)
    for branch in r:
        if branch.get("default"):
            result["HEAD"] = branch["commit"]["id"]
        result["refs/heads/%s" % branch["name"]] = branch["commit"]["id"]
    if tags:
        path = "%s/tags" % base_path
        r = gl.http_get(path)
        for tag in r:
            result["refs/tags/%s" % tag["name"]] = tag["commit"]["id"]
    return result
