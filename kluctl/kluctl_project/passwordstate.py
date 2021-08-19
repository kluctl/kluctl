import sys
from getpass import getpass
from urllib.parse import urlencode

import requests
from requests_ntlm import HttpNtlmAuth

from kluctl.utils.thread_safe_cache import ThreadSafeCache


class Passwordstate:
    session_cache = ThreadSafeCache()
    def get_session(self, host):
        def do_get_session():
            print(f"Please enter credentials for passwordstate host '{host}'", file=sys.stderr)
            sys.stderr.write("Username: ")
            username = input()
            password = getpass("Password: ")
            session = requests.Session()
            session.auth = HttpNtlmAuth(username=username, password=password)
            return session

        return self.session_cache.get(f"session-{host}", do_get_session)

    password_list_cache = ThreadSafeCache()
    def get_password_list(self, host, path):
        path = path.replace("/", "\\")
        name = path.split("\\")[-1]

        def do_get():
            session = self.get_session(host)
            url = f"https://{host}/winapi/searchpasswordlists/?"
            url += urlencode({
                "PasswordList": f"{name}"
            })
            r = session.get(url)
            r.raise_for_status()
            j = r.json()
            j = [x for x in j if x["PasswordList"] == name and x["TreePath"] == path]
            if len(j) == 0:
                raise Exception(f"No passwordlist found for {path}")
            if len(j) != 1:
                raise Exception(f"More then one passwordlist found for {path}")
            return j[0]
        return self.password_list_cache.get(f"{host}-{path}", do_get)

    password_cache = ThreadSafeCache()
    def get_password(self, host, password_list_id, title, field):
        def do_get():
            session = self.get_session(host)
            url = f"https://{host}/winapi/searchpasswords/{password_list_id}?"
            url += urlencode({
                "title": f"\"{title}\""
            })
            r = session.get(url)
            r.raise_for_status()
            j = r.json()
            if len(j) == 0:
                raise Exception(f"No password with title {title} found in passwordlist {password_list_id}")
            if len(j) != 1:
                raise Exception(f"More then one password with title {title} found in passwordlist {password_list_id}")
            if field not in j[0]:
                raise Exception(f"Password has no field with name '{field}'")
            return j[0][field]
        return self.password_cache.get(f"{host}-{password_list_id}-{title}-{field}", do_get)

    document_cache = ThreadSafeCache()
    def get_document(self, host, id):
        def do_get():
            session = self.get_session(host)
            url = f"https://{host}/winapi/document/passwordlist/{id}"
            r = session.get(url)
            r.raise_for_status()
            return r.text
        return self.document_cache.get(f"{host}-{id}", do_get)
