import functools
import json
import logging
import threading
import time
from distutils.version import StrictVersion

from kubernetes import config
from kubernetes.client import ApiClient, Configuration, ApiException
from kubernetes.config import ConfigException
from kubernetes.dynamic import EagerDiscoverer, DynamicClient, Resource
from kubernetes.dynamic.exceptions import NotFoundError, ResourceNotFoundError

from kluctl.utils.exceptions import CommandError
from kluctl.utils.k8s_cluster_base import k8s_cluster_base
from kluctl.utils.dict_utils import copy_dict, get_dict_value
from kluctl.utils.k8s_object_utils import split_api_version
from kluctl.utils.versions import LooseVersionComparator

logger = logging.getLogger(__name__)

discoverer_lock = threading.Lock()

deprecated_resources = {
    ("extensions", "Ingress")
}

class MyDiscoverer(EagerDiscoverer):
    did_invalidate_cache = False

    def invalidate_cache(self):
        with discoverer_lock:
            if self.did_invalidate_cache:
                return
            super().invalidate_cache()
            self.did_invalidate_cache = True

    def force_invalidate_cache(self):
        with discoverer_lock:
            self.did_invalidate_cache = False

# This is a hack to force the apiserver to return server-side tables. These also include the full metadata of each
# object, which we can then use to query for only metadata
class TableApiClient(ApiClient):
    def select_header_accept(self, accepts):
        return "application/json;as=Table;v=v1;g=meta.k8s.io,application/json;as=Table;v=v1beta1;g=meta.k8s.io,application/json"

class k8s_cluster_real(k8s_cluster_base):
    def __init__(self, context, dry_run):
        self.context = context
        self.dry_run = dry_run

        self.config = Configuration()
        try:
            config.load_kube_config(context=context, client_configuration=self.config, persist_config=True)
        except ConfigException as e:
            raise CommandError(str(e))

        if self.context != "docker-desktop" and not self.config.api_key and not self.config.key_file:
            raise CommandError("No authentication available. You might need to invoke kubectl first to perform a login")

        self.api_client = ApiClient(configuration=self.config)
        self.dynamic_client = DynamicClient(self.api_client, discoverer=MyDiscoverer)
        self.table_api_client = TableApiClient(configuration=self.config)
        self.table_dynamic_client = DynamicClient(self.table_api_client, discoverer=MyDiscoverer)

        v = self.dynamic_client.version
        self.server_version = "%s.%s" % (v['kubernetes']['major'], v['kubernetes']['minor'])

    def _fix_api_version(self, group, version):
        if group is None and version is not None:
            version = version.split('/', 2)
            if len(version) == 1:
                version = version[0]
            else:
                group = version[0]
                version = version[1]
        return group, version

    def _build_label_selector(self, labels):
        if labels is None:
            return None
        if isinstance(labels, list):
            return ','.join(labels)
        if isinstance(labels, dict):
            return ','.join(['%s=%s' % (k, v) for k, v in labels.items()])
        else:
            raise ValueError('Unsupported labels type')

    def get_status_message(self, status):
        if isinstance(status, ApiException):
            if status.headers.get("Content-Type") == "application/json":
                status = json.loads(status.body)
            else:
                return status.body.decode("utf-8")
        if isinstance(status, Exception):
            return 'Exception: %s' % str(status)

        message = 'Unknown'
        if 'message' in status:
            message = status['message']
        elif 'reason' in status:
            message = status['reason']
        return message

    def get_dry_run_suffix(self, force=False):
        if self.dry_run or force:
            return ' (dry-run)'
        return ''

    def _get_dry_run_params(self, force=False):
        params = []
        if self.dry_run or force:
            params.append(('dryRun', 'All'))
        return params

    def get_resource(self, group, version, kind):
        return self.dynamic_client.resources.get(group=group, api_version=version, kind=kind)

    def get_preferred_resources(self, group, api_version, kind):
        kinds = {}
        for r in self.dynamic_client.resources.search(group=group, api_version=api_version, kind=kind):
            if type(r) != Resource:
                continue
            if (r.group, r.kind) in deprecated_resources:
                continue
            key = (r.group or None, r.kind)
            kinds.setdefault(key, []).append(r)
        def cmp_resource(a, b):
            va = a.api_version
            vb = b.api_version
            if a.preferred:
                va = "100000000"
            if b.preferred:
                vb = "100000000"
            return LooseVersionComparator.compare(va, vb)
        for key in kinds.keys():
            kinds[key].sort(key=functools.cmp_to_key(cmp_resource))
            kinds[key] = kinds[key][-1]
        return list(kinds.values())

    def get_preferred_resource(self, group, kind):
        resources = self.get_preferred_resources(group, None, kind)
        for r in resources:
            if group is not None and group != (r.group or None):
                continue
            if kind is not None and kind != (r.kind or None):
                continue
            return r
        raise ResourceNotFoundError(f"{group}/{kind} not found")

    def _get_objects_for_resource(self, resource, name, namespace, labels, as_table):
        label_selector = self._build_label_selector(labels)

        logger.debug("GET resource=%s, name=%s, namespace=%s, labels=%s" % (resource, name, namespace, labels))

        try:
            client = self.dynamic_client
            if as_table:
                client = self.table_dynamic_client
            r = client.get(resource, name, namespace, serialize=False, label_selector=label_selector)
            warnings = r.headers.getlist("Warning")
            ret = json.loads(r.data)
            return ret, warnings
        except NotFoundError:
            return None

    def _get_objects(self, group, version, kind, name, namespace, labels, as_table):
        group, version = self._fix_api_version(group, version)

        resources = self.get_preferred_resources(group, version, kind)

        def fix_kind(resource, r, warnings):
            if not r:
                return []
            if name is not None:
                return [(r, warnings)]

            ret_group, ret_version = split_api_version(r.get("apiVersion"))
            if as_table and ret_group == "meta.k8s.io" and r["kind"] == "Table":
                r = r["rows"] or []
                r = [x["object"] for x in r]
            else:
                r = r["items"]
            if not r:
                return []

            for x in r:
                x['kind'] = resource.kind
                x['apiVersion'] = resource.group_version
            r = [(x, warnings) for x in r]
            return r

        ret = []
        for resource in resources:
            r = self._get_objects_for_resource(resource, name, namespace, labels, as_table)
            if r is None:
                continue
            objects, warnings = r
            ret += fix_kind(resource, objects, warnings)

        return ret

    def patch_object(self, body, namespace=None, force_dry_run=False, force_apply=False):
        namespace = namespace or body.get('metadata', {}).get('namespace')
        group, version = split_api_version(body.get("apiVersion"))
        kind = body['kind']
        name = body['metadata']['name']
        query_params = self._get_dry_run_params(force_dry_run)

        query_params.append(('fieldManager', 'kluctl'))
        if force_apply:
            query_params.append(('force', 'true'))

        resource = self.dynamic_client.resources.get(group=group, api_version=version, kind=kind)
        if resource.namespaced:
            self.dynamic_client.ensure_namespace(resource, namespace, body)

        body = json.dumps(body)

        logger.debug("PATCH resource=%s, name=%s, namespace=%s" % (resource, name, namespace))
        r = self.dynamic_client.patch(resource, body=body, name=name, namespace=namespace, serialize=False,
                                      query_params=query_params, content_type='application/apply-patch+yaml')
        warnings = r.headers.getlist("Warning")
        r = json.loads(r.data)
        if not force_dry_run and not self.dry_run and kind == "CustomResourceDefinition":
            self.dynamic_client.resources.force_invalidate_cache()
        return r, warnings

    def replace_object(self, body, namespace=None, force_dry_run=False, resource_version=None):
        namespace = namespace or body.get('metadata', {}).get('namespace')
        resource_version = resource_version or get_dict_value(body, "metadata.resourceVersion")
        group, version = split_api_version(body.get("apiVersion"))
        kind = body['kind']
        name = body['metadata']['name']
        query_params = self._get_dry_run_params(force_dry_run)

        query_params.append(('fieldManager', 'kluctl'))

        resource = self.dynamic_client.resources.get(group=group, api_version=version, kind=kind)
        if resource.namespaced:
            self.dynamic_client.ensure_namespace(resource, namespace, body)

        body = json.dumps(body)

        logger.debug("PUT resource=%s, name=%s, namespace=%s" % (resource, name, namespace))
        r = self.dynamic_client.replace(resource, body=body, name=name, namespace=namespace, serialize=False,
                                        query_params=query_params, content_type='application/yaml',
                                        resource_version=resource_version)
        warnings = r.headers.getlist("Warning")
        r = json.loads(r.data)
        if not force_dry_run and not self.dry_run and kind == "CustomResourceDefinition":
            self.dynamic_client.resources.force_invalidate_cache()
        return r, warnings

    def fix_object_for_patch(self, o):
        # A bug in versions < 1.20 cause errors when applying resources that have some fields omitted which have
        # default values. We need to fix these resources.
        # UPDATE even though https://github.com/kubernetes-sigs/structured-merge-diff/issues/130 says it's fixed, the
        # issue is still present.
        needs_defaults_fix = StrictVersion(self.server_version) < StrictVersion('1.100')
        # TODO check when this is actually fixed (see https://github.com/kubernetes/kubernetes/issues/94275)
        needs_type_conversion_fix = StrictVersion(self.server_version) < StrictVersion('1.100')
        if not needs_defaults_fix and not needs_type_conversion_fix:
            return o

        o = copy_dict(o)

        def fix_ports(ports):
            if not needs_defaults_fix:
                return
            for p in ports:
                if 'protocol' not in p:
                    p['protocol'] = 'TCP'

        def fix_string_type(d, k):
            if d is None:
                return
            if not needs_type_conversion_fix:
                return
            if k not in d:
                return
            if not isinstance(d[k], str):
                d[k] = str(d[k])

        def fix_container(c):
            fix_ports(c.get('ports', []))
            fix_string_type((c.get('resources') or {}).get('limits', None), 'cpu')
            fix_string_type((c.get('resources') or {}).get('requests', None), 'cpu')

        def fix_containers(containers):
            for c in containers:
                fix_container(c)

        fix_containers(o.get('spec', {}).get('template', {}).get('spec', {}).get('containers', []))
        fix_ports(o.get('spec', {}).get('ports', []))
        for x in o.get('spec', {}).get('limits', []):
            fix_string_type(x.get('default'), 'cpu')
            fix_string_type(x.get('defaultRequest'), 'cpu')
        return o

    def delete_single_object(self, ref, force_dry_run=False, cascade="Foreground", ignore_not_found=False, do_wait=True):
        group, version = self._fix_api_version(None, ref.api_version)
        dry_run = self.dry_run or force_dry_run

        resource = self.dynamic_client.resources.get(group=group, api_version=version, kind=ref.kind)

        body = {
            "kind": "DeleteOptions",
            "apiVersion": "v1",
            "propagationPolicy": cascade,
        }
        if dry_run:
            body["dryRun"] = ["All"]
        body = json.dumps(body)

        logger.debug("DELETE resource=%s, name=%s, namespace=%s" % (resource, ref.name, ref.namespace))
        try:
            r = self.dynamic_client.delete(resource, ref.name, ref.namespace, serialize=False, body=body, content_type='application/yaml')
            # We need to ensure that the object is actually deleted, as the DELETE request is returning early
            if not dry_run and do_wait:
                self._wait_for_deleted_object(ref)
        except NotFoundError as e:
            if not ignore_not_found:
                raise e
            return None
        return json.loads(r.data)

    def _wait_for_deleted_object(self, ref):
        while True:
            try:
                o, _ = self.get_single_object(ref)
                if not o:
                    break
            except:
                break
            time.sleep(0.2)
