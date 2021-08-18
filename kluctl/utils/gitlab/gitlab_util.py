import os.path
from urllib.parse import quote

import gitlab
from urllib3.util import parse_url

from kluctl.utils.session_pool import get_session_from_pool
from kluctl.utils.yaml_utils import yaml_load

gitlab_url = "https://gitlab.com"
gitlab_token = None

def init_gitlab_util(url, token):
    global gitlab_url
    global gitlab_token
    if url is not None:
        gitlab_url = url
    if token is not None:
        gitlab_token = token

def init_gitlab_util_from_glab():
    try:
        with open(os.path.expanduser("~/.config/glab-cli/config.yml")) as f:
            buf = f.read()
            # some strange protection applied by glab?
            buf = buf.replace("token: !!null ", "token: ")
            y = yaml_load(buf)

        for h in y.get("hosts", []).values():
            if h.get("api_host") == "gitlab.com":
                token = h.get("token")
                init_gitlab_util("https://gitlab.com", token)
                return
    except:
        pass

def get_gitlab_api(job_token=None, require_auth=None):
    kwargs = {}
    if job_token is not None:
        kwargs["job_token"] = job_token
    else:
        if gitlab_token:
            kwargs["private_token"] = gitlab_token
        elif require_auth:
            return None

    kwargs["session"] = get_session_from_pool(("gitlab", job_token, require_auth))
    return gitlab.Gitlab(gitlab_url, **kwargs)

def is_gitlab_project(git_url):
    from kluctl.utils.git_utils import parse_git_url
    gitlab_host = parse_url(gitlab_url).host

    try:
        u = parse_git_url(git_url)
        return u.host == gitlab_host
    except:
        return False

def extract_gitlab_group_and_project(git_url):
    from kluctl.utils.git_utils import parse_git_url
    assert is_gitlab_project(git_url)

    u = parse_git_url(git_url)
    p = u.path.split("/")
    group = "/".join(p[:-1])
    project = p[-1]
    return group, project

def build_gitlab_project_id(group, name):
    id = quote("%s/%s" % (group, name), safe="")
    return id
