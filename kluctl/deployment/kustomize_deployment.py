import hashlib
import logging
import os
import shutil

from kluctl.utils.dict_utils import merge_dict
from kluctl.utils.exceptions import CommandError
from kluctl.deployment.helm_chart import HelmChart
from kluctl.seal.deployment_sealer import SEALME_EXT
from kluctl.utils.external_tools import get_external_tool_hash
from kluctl.utils.k8s_object_utils import should_remove_namespace, get_object_ref
from kluctl.utils.kustomize import kustomize_build
from kluctl.utils.templated_dir import TemplatedDir
from kluctl.utils.utils import get_tmp_base_dir, calc_dir_hash
from kluctl.utils.versions import LooseSemVerLatestVersion, PrefixLatestVersion, NumberLatestVersion, \
    RegexLatestVersion
from kluctl.utils.yaml_utils import yaml_load_file, yaml_load_all, yaml_save_file

logger = logging.getLogger(__name__)

def get_kustomize_cache_dir():
    dir = os.path.join(get_tmp_base_dir(), "kluctl-kustomize-cache")
    os.makedirs(dir, exist_ok=True)
    return dir

allow_cache = True

class KustomizeDeployment(object):
    def __init__(self, deployment_project, deployment_collection, config):
        self.deployment_project = deployment_project
        self.deployment_collection = deployment_collection
        self.config = config
        self.objects = None

    def get_rel_kustomize_dir(self):
        return self.deployment_project.get_rel_dir_to_root(self.config["path"])

    def get_rendered_dir(self):
        path = self.get_rel_kustomize_dir()
        rendered_dir = os.path.join(self.deployment_collection.tmpdir, path)
        return rendered_dir

    def get_common_labels(self):
        l = self.deployment_project.get_common_labels()
        for i, tag in enumerate(self.get_tags()):
            l['kluctl.io/tag-%d' % i] = tag
        return l

    def get_common_annotations(self):
        a = {
            # Must use annotations instead of labels due to limitations on value (max 63 chars, no slash)
            "kluctl.io/kustomize_dir": self.get_rel_kustomize_dir().replace("\\", "/")
        }
        if self.config.get("skipDeleteIfTags", False):
            a["kluctl.io/skip-delete-if-tags"] = "true"
        return a

    def build_images_jinja_vars(self):
        if self.deployment_collection.images is None:
            return {}

        tags = list(sorted(self.get_tags()))

        return {
            'images': {
                'get_image': self.deployment_collection.seen_images.get_image_wrapper(self.get_rel_kustomize_dir(), tags),
            },
            'version': {
                'semver': LooseSemVerLatestVersion,
                'prefix': PrefixLatestVersion,
                'number': NumberLatestVersion,
                'regex': RegexLatestVersion,
            },
        }

    def build_jinja2_env(self, jinja_vars):
        images_vars = self.build_images_jinja_vars()
        jinja_vars = merge_dict(jinja_vars, images_vars)
        merge_dict(jinja_vars, {
            "deployment": {
                "tags": list(self.get_tags())
            }
        }, clone=False)
        return self.deployment_project.build_jinja2_env(jinja_vars)

    def render(self, executor):
        if "path" not in self.config:
            return []

        path = self.config["path"]
        rendered_dir = self.get_rendered_dir()

        os.makedirs(rendered_dir, exist_ok=True)

        jinja_vars = self.deployment_project.jinja_vars
        jinja_env = self.build_jinja2_env(jinja_vars)

        excluded_patterns = self.deployment_project.conf['templateExcludes'].copy()
        if os.path.exists(os.path.join(self.deployment_project.dir, path, 'helm-chart.yml')):
            # never try to render helm charts
            excluded_patterns.append(os.path.join(path, 'charts/*'))
        if not self.deployment_collection.for_seal:
            # .sealme files are rendered while sealing and not while deploying
            excluded_patterns.append('*.sealme')

        d = TemplatedDir(self.deployment_project.dir, jinja_env, executor, excluded_patterns)
        return d.async_render_subdir(path, rendered_dir)

    def render_helm_charts(self, executor):
        if "path" not in self.config:
            return []

        jobs = []
        rendered_dir = self.get_rendered_dir()
        for dirpath, dirnames, filenames in os.walk(rendered_dir):
            path = os.path.join(dirpath, 'helm-chart.yml')
            if not os.path.exists(path):
                continue
            chart = HelmChart(path)
            job = executor.submit(chart.render)
            jobs.append(job)
        return jobs

    def resolve_sealed_secrets(self):
        if "path" not in self.config:
            return

        sealed_secrets_dir = self.deployment_project.get_sealed_secrets_dir(self.config["path"])
        rel_dir_to_root = self.deployment_project.get_rel_dir_to_root()
        rendered_dir = self.get_rendered_dir()
        base_source_path = os.path.join(self.deployment_project.sealed_secrets_dir, rel_dir_to_root)

        y = yaml_load_file(os.path.join(rendered_dir, "kustomization.yml")) or {}
        for resource in y.get("resources", []):
            p = os.path.join(rendered_dir, resource)
            if os.path.exists(p) or not os.path.exists(p + SEALME_EXT):
                continue
            rel_dir = os.path.relpath(os.path.dirname(p), rendered_dir)
            fname = os.path.basename(p)

            base_error = 'Failed to resolve SealedSecret %s' % os.path.normpath(os.path.join(self.deployment_project.dir, resource))
            if sealed_secrets_dir is None:
                raise CommandError('%s\nSealed secrets dir could not be determined.' % base_error)
            source_path = os.path.normpath(os.path.join(base_source_path, self.config["path"], rel_dir, sealed_secrets_dir, fname))
            target_path = os.path.join(rendered_dir, rel_dir, fname)
            if not os.path.exists(source_path):
                raise CommandError('%s\n%s not found.\nYou might need to seal it first.' % (base_error, source_path))
            shutil.copy(source_path, target_path)

    def get_tags(self):
        tags = self.deployment_project.get_tags()

        for t in self.config.get("tags", []):
            tags.add(t)

        return tags

    def build_inclusion_values(self):
        values = [("tag", tag) for tag in self.get_tags()]
        if "path" in self.config:
            kustomize_dir = self.get_rel_kustomize_dir()
            values.append(("kustomize_dir", kustomize_dir))
        return values

    def check_inclusion_for_deploy(self):
        inclusion = self.deployment_collection.inclusion
        if inclusion is None:
            return True
        if self.config.get("onlyRender", False):
            return True
        if self.config.get("alwaysDeploy", False):
            return True
        values = self.build_inclusion_values()
        return inclusion.check_included(values)

    def check_inclusion_for_delete(self):
        inclusion = self.deployment_collection.inclusion
        if inclusion is None:
            return True
        skip_delete_if_tags = self.config.get("skipDeleteIfTags", False)
        values = self.build_inclusion_values()
        return inclusion.check_included(values, skip_delete_if_tags)

    def build_objects_kustomize(self):
        tmp_files = []

        try:
            need_build = True
            if allow_cache:
                hash = self.calc_hash()
                hash_path = os.path.join(get_kustomize_cache_dir(), hash[:16])
                if os.path.exists(hash_path):
                    need_build = False
                    # Load from cache
                    with open(hash_path) as f:
                        yamls = f.read()
            if need_build:
                # Run 'kustomize build'
                yamls = kustomize_build(self.get_rendered_dir())
                if allow_cache:
                    with open(hash_path + "~", "w") as f:
                        f.write(yamls)
                    os.rename(hash_path + "~", hash_path)
        finally:
            for x in tmp_files:
                x.close()

        yamls = yaml_load_all(yamls)
        return yamls

    def build_objects(self, k8s_cluster):
        if self.config.get("onlyRender", False):
            self.objects = []
            return
        if "path" not in self.config:
            self.objects = []
            return

        rendered_dir = self.get_rendered_dir()
        kustomize_yaml_path = os.path.join(rendered_dir, 'kustomization.yml')
        kustomize_yaml = yaml_load_file(kustomize_yaml_path)

        override_namespace = self.deployment_project.get_override_namespace()
        if override_namespace is not None:
            kustomize_yaml.setdefault("namespace", override_namespace)

        # Save modified kustomize.yml
        yaml_save_file(kustomize_yaml, kustomize_yaml_path)

        yamls = self.build_objects_kustomize()

        self.objects = []
        # Add commonLabels to all resources. We can't use kustomize's "commonLabels" feature as it erroneously
        # sets matchLabels as well.
        for y in yamls:
            labels = y.setdefault("metadata", {}).setdefault("labels") or {}
            annotations = y["metadata"].setdefault("annotations") or {}
            labels.update(self.get_common_labels())
            annotations.update(self.get_common_annotations())
            y["metadata"]["labels"] = labels
            y["metadata"]["annotations"] = annotations

            if should_remove_namespace(k8s_cluster, get_object_ref(y)):
                del y["metadata"]["namespace"]

            self.objects.append(y)

    def calc_hash(self):
        h = hashlib.sha256()
        h.update(get_external_tool_hash("kustomize").encode("utf-8"))
        h.update(calc_dir_hash(self.get_rendered_dir()).encode("utf-8"))
        return h.hexdigest()
