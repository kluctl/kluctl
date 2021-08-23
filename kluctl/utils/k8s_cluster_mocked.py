from kluctl.utils.k8s_cluster_base import k8s_cluster_base
from kluctl.utils.k8s_object_utils import split_api_version


class k8s_cluster_mocked(k8s_cluster_base):
    def __init__(self):
        self.objects = []

    def add_object(self, o):
        self.objects.append(o)

    def _get_objects(self, group, version, kind, name, namespace, labels, as_table):
        if labels is None:
            labels = {}

        ret = []
        for o in self.objects:
            o_namespace = o['metadata']['namespace'] if 'namespace' in o['metadata'] else None
            o_group, o_version = split_api_version(o.get("apiVersion"))
            o_kind = o['kind']
            o_name = o['metadata']['name']
            o_labels = o['metadata']['labels'] if 'labels' in o['metadata'] else {}

            if namespace != o_namespace:
                continue
            if group is not None and group != o_group:
                continue
            if version is not None and version != o_version:
                continue
            if kind is not None and kind != o_kind:
                continue
            if name is not None and name != o_name:
                continue

            labels_ok = True
            for n, v in labels.items():
                if n not in o_labels or v != o_labels[n]:
                    labels_ok = False
                    break
            if not labels_ok:
                continue
            ret.append((o, []))
        return ret
