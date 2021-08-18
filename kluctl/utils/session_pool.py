import threading

import requests

mutex = threading.Lock()
local = threading.local()
session_pool = {}

class LocalSessionHolder:
    def __init__(self, key, session):
        self.key = key
        self.session = session

    def __del__(self):
        with mutex:
            # release it to the global pool
            session_pool.setdefault(self.key, []).append(self.session)

def get_session_from_pool(key):
    try:
        d = local.sessions
    except:
        local.sessions = {}
        d = local.sessions

    if key not in d:
        with mutex:
            pool = session_pool.get(key)
            if pool:
                s = pool.pop()
            else:
                s = requests.Session()
        d[key] = LocalSessionHolder(key, s)

    return d[key].session
