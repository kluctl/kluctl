import hashlib
import logging
import os
import shutil

from kluctl.deployment.helm_chart import HelmChart
from kluctl.seal.deployment_sealer import SEALME_EXT
from kluctl.utils.dict_utils import merge_dict
from kluctl.utils.exceptions import CommandError, InvalidKluctlProjectConfig
from kluctl.utils.external_tools import get_external_tool_hash
from kluctl.utils.k8s_object_utils import get_object_ref, should_remove_namespace
from kluctl.utils.kustomize import kustomize_build
from kluctl.utils.templated_dir import TemplatedDir
from kluctl.utils.utils import get_tmp_base_dir, calc_dir_hash
from kluctl.utils.versions import LooseSemVerLatestVersion, PrefixLatestVersion, NumberLatestVersion, \
    RegexLatestVersion
from kluctl.utils.yaml_utils import yaml_load_file, yaml_save_file

logger = logging.getLogger(__name__)

def get_kustomize_cache_dir():
    dir = os.path.join(get_tmp_base_dir(), "kluctl-kustomize-cache")
    os.makedirs(dir, exist_ok=True)
    return dir

allow_cache = True

class KustomizeDeployment(object):
    def __init__(self, deployment_project, deployment_collection, config, dir, index):
        self.deployment_project = deployment_project
        self.deployment_collection = deployment_collection
        self.config = config
        self.dir = dir
        self.index = index
        self.objects = []

        if dir and not os.path.isdir(dir):
            raise InvalidKluctlProjectConfig("kustomizeDir does not exist: %s" % dir)

    def get_rel_kustomize_dir(self):
        root_project = self.deployment_project.get_root_deployment()
        return os.path.relpath(self.dir, root_project.dir)

    def get_rel_rendered_dir(self):
        path = self.get_rel_kustomize_dir()
        if self.index != 0:
            path = "%s-%d" % (path, self.index)
        return path

    def get_rendered_dir(self):
        path = self.get_rel_rendered_dir()
        rendered_dir = os.path.join(self.deployment_collection.tmpdir, path)
        return rendered_dir

    def get_rendered_yamls_path(self):
        return os.path.join(self.get_rendered_dir(), ".rendered.yml")

    def get_common_labels(self):
        l = self.deployment_project.get_common_labels()
        for i, tag in enumerate(self.get_tags()):
            l['kluctl.io/tag-%d' % i] = tag
        return l

    def get_common_annotations(self):
        a = {
            # Must use annotations instead of labels due to limitations on value (max 63 chars, no slash)
            "kluctl.io/kustomize_dir": self.get_rel_kustomize_dir().replace(os.path.sep, "/")
        }
        if self.config.get("skipDeleteIfTags", False):
            a["kluctl.io/skip-delete-if-tags"] = "true"
        return a

    def get_image_wrapper(self, image, namespace=None, deployment_name=None, container=None, latest_version=LooseSemVerLatestVersion()):
        # TODO remove this
        if namespace is not None or deployment_name is not None or container is not None:
            logger.warning("images.get_image called with obsolete parameters (namespace/deployment_name/container). These are not needed anymore.")

        tags = list(sorted(self.get_tags()))
        return self.deployment_collection.images.gen_image_placeholder(image, latest_version, self.get_rel_kustomize_dir(), tags)

    def build_images_jinja_vars(self):
        if self.deployment_collection.images is None:
            return {}

        return {
            'images': {
                'get_image': self.get_image_wrapper,
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

    def render(self, k8s_cluster, executor):
        if self.dir is None:
            return []

        root_dir = self.deployment_project.get_root_deployment().dir
        rel_deployment_dir = os.path.relpath(self.deployment_project.dir, root_dir)
        rel_kustomize_dir = os.path.relpath(self.get_rel_kustomize_dir(), rel_deployment_dir)
        abs_rendered_dir = self.get_rendered_dir()

        os.makedirs(abs_rendered_dir, exist_ok=True)

        jinja_vars = self.deployment_project.jinja_vars
        if "vars" in self.config:
            jinja_vars = self.deployment_project.load_jinja_vars_list(k8s_cluster, self.config["vars"], jinja_vars)

        jinja_env = self.build_jinja2_env(jinja_vars)

        excluded_patterns = self.deployment_project.conf['templateExcludes'].copy()
        if os.path.exists(os.path.join(self.dir, 'helm-chart.yml')):
            # never try to render helm charts
            excluded_patterns.append(os.path.join(rel_kustomize_dir, 'charts/*'))
        if not self.deployment_collection.for_seal:
            # .sealme files are rendered while sealing and not while deploying
            excluded_patterns.append('*.sealme')

        d = TemplatedDir(root_dir, rel_deployment_dir, jinja_env, executor, excluded_patterns)
        return d.async_render_subdir(rel_kustomize_dir, abs_rendered_dir)

    def render_helm_charts(self, k8s_cluster, executor):
        if self.dir is None:
            return []

        jobs = []
        rendered_dir = self.get_rendered_dir()
        for dirpath, dirnames, filenames in os.walk(rendered_dir):
            path = os.path.join(dirpath, 'helm-chart.yml')
            if not os.path.exists(path):
                continue
            chart = HelmChart(path)
            job = executor.submit(chart.render, k8s_cluster)
            jobs.append(job)
        return jobs

    def resolve_sealed_secrets(self):
        if self.dir is None:
            return

        sealed_secrets_dir = self.deployment_project.get_sealed_secrets_dir()
        rel_rendered_dir = self.get_rel_rendered_dir()
        rendered_dir = self.get_rendered_dir()
        base_source_path = self.deployment_project.sealed_secrets_dir

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
            source_path = os.path.normpath(os.path.join(base_source_path, rel_rendered_dir, rel_dir, sealed_secrets_dir, fname))
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
        if self.dir is not None:
            kustomize_dir = self.get_rel_kustomize_dir().replace(os.path.sep, "/")
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

    def prepare_kustomization_yaml(self):
        if self.dir is None:
            return

        rendered_dir = self.get_rendered_dir()
        kustomize_yaml_path = os.path.join(rendered_dir, 'kustomization.yml')
        if not os.path.exists(kustomize_yaml_path):
            return
        kustomize_yaml = yaml_load_file(kustomize_yaml_path)

        override_namespace = self.deployment_project.get_override_namespace()
        if override_namespace is not None:
            kustomize_yaml.setdefault("namespace", override_namespace)

        # Save modified kustomize.yml
        yaml_save_file(kustomize_yaml, kustomize_yaml_path)

    def build_kustomize(self):
        if self.dir is None:
            return

        self.prepare_kustomization_yaml()

        need_build = True
        if allow_cache:
            hash = self.calc_hash()
            hash_path = os.path.join(get_kustomize_cache_dir(), hash[:16])
            if os.path.exists(hash_path):
                need_build = False
                # Copy from cache
                shutil.copy(hash_path, self.get_rendered_yamls_path())
        if need_build:
            # Run 'kustomize build'
            yamls = kustomize_build(self.get_rendered_dir())
            with open(self.get_rendered_yamls_path(), "wt") as f:
                f.write(yamls)
            if allow_cache:
                shutil.copy(self.get_rendered_yamls_path(), hash_path)

    def postprocess_and_load_objects(self, k8s_cluster):
        if self.dir is None:
            return

        self.objects = yaml_load_file(self.get_rendered_yamls_path(), all=True)
        for y in self.objects:
            ref = get_object_ref(y)
            if k8s_cluster is not None and should_remove_namespace(k8s_cluster, ref):
                del y["metadata"]["namespace"]
                ref = get_object_ref(y)

            # Set common labels/annotations
            labels = y.setdefault("metadata", {}).setdefault("labels") or {}
            annotations = y["metadata"].setdefault("annotations") or {}
            labels.update(self.get_common_labels())
            annotations.update(self.get_common_annotations())
            y["metadata"]["labels"] = labels
            y["metadata"]["annotations"] = annotations

            # Resolve image placeholders
            s = str(y)
            if any(p in s for p in self.deployment_collection.images.placeholders.keys()):
                if k8s_cluster is not None:
                    remote_object, warnings = k8s_cluster.get_single_object(ref)
                else:
                    remote_object = None
                self.deployment_collection.images.resolve_placeholders(y, remote_object)

        # Need to write it back to disk in case it is needed externally
        yaml_save_file(self.objects, self.get_rendered_yamls_path())

    def calc_hash(self):
        h = hashlib.sha256()
        h.update(get_external_tool_hash("kustomize").encode("utf-8"))
        h.update(calc_dir_hash(self.get_rendered_dir()).encode("utf-8"))
        return h.hexdigest()
