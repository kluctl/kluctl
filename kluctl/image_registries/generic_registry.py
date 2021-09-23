import base64
import contextlib
import hashlib
import http
import json
import logging
import os
import socket
import threading
from calendar import timegm
from collections import namedtuple
from datetime import datetime

import jwt
import requests
from dxf import DXF
from dxf.exceptions import DXFUnauthorizedError
from jwt import InvalidTokenError
from requests import HTTPError

from kluctl.image_registries.images_registry import ImagesRegistry
from kluctl.utils.exceptions import CommandError
from kluctl.utils.run_helper import run_helper
from kluctl.utils.thread_safe_cache import ThreadSafeCache, ThreadSafeMultiCache
from kluctl.utils.utils import get_tmp_base_dir

logger = logging.getLogger(__name__)

DOCKER_HUB_REGISTRY = "registry-1.docker.io"
DOCKER_HUB_REGISTRY2 = "docker.io"
DOCKER_HUB_AUTH_ENTRY = "https://index.docker.io/v1/"
USER_AGENT = "Docker-Client/19.03.2 (linux)'"

DockerCreds = namedtuple("DockerCreds", "username password tlsverify")

class GenericRegistry(ImagesRegistry):
    def __init__(self, default_tlsverify):
        self.default_tlsverify = default_tlsverify
        self.creds = {}
        self.creds_cache = ThreadSafeMultiCache()
        self.dns_cache = ThreadSafeCache()
        self.info_cache = ThreadSafeMultiCache()
        self.client_cache = ThreadSafeMultiCache()
        self.first_auth_cache = ThreadSafeCache()

        self.shared_session = requests.Session()
        adapter = requests.adapters.HTTPAdapter(pool_connections=4, pool_maxsize=16)
        self.shared_session.mount('https://', adapter)

    def add_creds(self, host, username, password, tlsverify):
        if host is None:
            host = DOCKER_HUB_REGISTRY
        self.creds[host] = DockerCreds(username=username, password=password, tlsverify=tlsverify)

    def do_get_creds(self, host):
        logger.debug(f"looking up {host} in creds")
        if host in self.creds:
            logger.debug(f"found entry with username {self.creds[host].username} in creds")
            return self.creds[host]

        logger.debug("entry not found in creds, looking into docker config")

        auth_entry = host
        if host == DOCKER_HUB_REGISTRY:
            auth_entry = DOCKER_HUB_AUTH_ENTRY

        # try from docker creds
        try:
            with open(os.path.expanduser("~/.docker/config.json")) as f:
                config = json.load(f)
            auth = config.get("auths", {}).get(auth_entry)
            if auth is None:
                return None
            if "username" in auth and "password" in auth:
                logger.debug(f"found docker config entry with username {auth['username']}")
                return DockerCreds(username=auth["username"], password=auth["password"], tlsverify=None)
            if "auth" in auth:
                a = base64.b64decode(auth["auth"]).decode("utf-8")
                a = a.split(":", 1)
                if len(a) != 2:
                    logger.debug(f"don't know how to handle auth entry in docker config with len={len(a)}")
                    return None
                logger.debug(f"found docker config entry with username {a[0]}")
                return DockerCreds(username=a[0], password=a[1], tlsverify=None)
            cred_store = config.get("credsStore")
            if cred_store is not None:
                cred_exe = f"docker-credential-{cred_store}"
                logger.debug(f"trying credStore {cred_exe}")
                rc, stdout, stderr = run_helper([cred_exe, "get"], input=auth_entry, return_std=True)
                if rc != 0:
                    logger.debug(f"{cred_exe} exited with status {rc}")
                    return None
                j = json.loads(stdout)
                logger.debug(f"{cred_exe} returned auth entry with username {j['Username']}")
                return DockerCreds(username=j["Username"], password=j["Secret"], tlsverify=None)
        except:
            pass
        return None

    def get_creds(self, host):
        return self.creds_cache.get(host, "creds", lambda: self.do_get_creds(host))

    def get_dns_info(self, n):
        try:
            return self.dns_cache.get(n, lambda: socket.gethostbyname(n))
        except:
            return None

    def get_info_response2(self, host, path):
        headers = {
            "User-Agent": USER_AGENT,
        }

        def do_get_info():
            tlsverify = None
            if host in self.creds:
                tlsverify = self.creds[host][2]
            if tlsverify is None:
                tlsverify = self.default_tlsverify
            url = f"https://{host}{path}"
            r = self.shared_session.get(url, headers=headers, verify=tlsverify)
            _ = r.content
            r.close()
            logger.debug(f"GET for {url} returned status {r.status_code} and headers {r.headers}")
            return r

        return self.info_cache.get((host, path), "info", do_get_info)

    def get_info_response(self, host, repo):
        r = self.get_info_response2(host, "/v2")
        if r.status_code == 404:
            r = self.get_info_response2(host, f"/v2/{repo}/tags/list")
        return r

    def parse_image(self, image):
        s = image.split("/")
        if s[0] == DOCKER_HUB_REGISTRY2:
            s[0] = DOCKER_HUB_REGISTRY
        if self.get_dns_info(s[0]) is not None:
            return s[0], "/".join(s[1:])
        else:
            return DOCKER_HUB_REGISTRY, image

    def get_cached_token_path(self, image, username, password):
        hash = hashlib.sha256(f"{image}_{username}_{password}".encode("utf-8")).hexdigest()
        path = os.path.join(get_tmp_base_dir(), "registry-tokens", hash)
        return path

    def parse_token(self, token):
        try:
            return jwt.decode(token,
                              None,
                              algorithms=["RS256"],
                              options={"verify_signature": False}
                              )
        except InvalidTokenError as e:
            return None

    def get_cached_token(self, image, username, password):
        path = self.get_cached_token_path(image, username, password)
        if not os.path.exists(path):
            return None
        with open(path, "r+t") as f:
            token = f.read()
        data = self.parse_token(token)
        try:
            exp = int(data["exp"])
            now = timegm(datetime.utcnow().utctimetuple())
            leeway = 10
            if exp - leeway < now:
                raise Exception("Signature has expired")
            return token
        except Exception:
            os.unlink(path)
            return None

    def set_cached_token(self, image, username, password, token):
        # ensure we can actually parse it (jwt compliant)
        if self.parse_token(token) is None:
            return

        path = self.get_cached_token_path(image, username, password)
        os.makedirs(os.path.dirname(path), exist_ok=True)
        with open(path, "wt") as f:
            os.chmod(path, 0o600)
            f.write(token)

    def get_client(self, image):
        host, repo = self.parse_image(image)
        if host == DOCKER_HUB_REGISTRY:
            if len(repo.split("/")) == 1:
                repo = "library/" + repo

        creds = self.get_creds(host)
        tlsverify = None
        if creds is not None:
            tlsverify = creds.tlsverify
        if tlsverify is None:
            tlsverify = self.default_tlsverify

        username = creds.username if creds else None
        password = creds.password if creds else None

        @contextlib.contextmanager
        def first_auth_context():
            first = self.first_auth_cache.get((host, username, password), lambda: {"lock": threading.Lock(), "first": True, "exception": None})
            first["lock"].acquire()
            has_lock = True
            try:
                if not first["first"]:
                    has_lock = False
                    first["lock"].release()
                    if first["exception"] is not None:
                        raise first["exception"]
                first["first"] = False
                yield None
            except Exception as e:
                if has_lock:
                    first["exception"] = e
                raise e
            finally:
                if has_lock:
                    first["lock"].release()

        def do_authenticate(dxf, response):
            token = self.get_cached_token(image, username, password)
            if token is not None:
                dxf.token = token
                return token
            with first_auth_context():
                logger.debug(f"calling dxf.authenticate with username={username}")
                base_auth_error = f"Got 401 Unauthorized from registry. Please ensure you provided correct registry " \
                                  f"credentials for '{host}', either by logging in with 'docker login {host}' or by " \
                                  f"providing environment variables as described in the documentation."
                try:
                    token = dxf.authenticate(username=username, password=password, response=response, actions=["pull"])
                except DXFUnauthorizedError:
                    raise CommandError(f"Got 401 Unauthorized from registry. {base_auth_error}")
                except HTTPError as e:
                    if e.response.status_code == http.HTTPStatus.FORBIDDEN:
                        raise CommandError(f"Got 403 Forbidden from registry. {base_auth_error}")
                    else:
                        raise
            if token is not None:
                self.set_cached_token(image, username, password, token)

        def do_get_client():
            info_response = self.get_info_response(host, repo)
            dxf = DXF(host, repo, tlsverify=tlsverify, auth=do_authenticate)
            # This is not how it was intended to be used, but it's the most effective way...
            dxf._sessions[0] = self.shared_session
            do_authenticate(dxf, info_response)
            return dxf

        return self.client_cache.get(image, "client", do_get_client)

    def is_image_from_registry(self, image):
        return True

    def list_tags_for_image(self, image):
        dxf = self.get_client(image)
        tags = dxf.list_aliases(batch_size=100)
        return tags
