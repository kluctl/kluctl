from git import Git


def git_ls_remote(url, tags=False):
    args = []
    if tags:
        args.append("--tags")
    args.append(url)
    g = Git()
    txt = g.ls_remote("-q", *args)
    lines = txt.splitlines()
    ret = {}
    for l in lines:
        x = l.split()
        ret[x[1]] = x[0]
    return ret
