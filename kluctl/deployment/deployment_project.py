import logging
import os

from jinja2 import StrictUndefined, FileSystemLoader
from ordered_set import OrderedSet

from kluctl.utils.dict_utils import merge_dict, copy_dict, \
    set_dict_default_value, get_dict_value
from kluctl.utils.exceptions import CommandError
from kluctl.utils.external_args import check_required_args
from kluctl.utils.jinja2_cache import KluctlBytecodeCache
from kluctl.utils.jinja2_utils import add_jinja2_filters, KluctlJinja2Environment
from kluctl.utils.k8s_object_utils import ObjectRef, get_long_object_name_from_ref
from kluctl.utils.yaml_utils import yaml_load, yaml_load_file

logger = logging.getLogger(__name__)


class DeploymentProject(object):
    def __init__(self, k8s_cluster, dir, jinja_vars, deploy_args, sealed_secrets_dir,
                 parent_collection=None):
        if not os.path.exists(dir):
            raise CommandError("%s does not exist" % dir)

        self.jinja2_cache = KluctlBytecodeCache(max_cache_files=10000)

        self.dir = dir
        self.jinja_vars = copy_dict(jinja_vars)
        self.deploy_args = deploy_args
        self.sealed_secrets_dir = sealed_secrets_dir
        self.jinja_vars['args'] = self.deploy_args
        self.includes = []
        self.parent_collection = parent_collection
        self.parent_collection_include = None
        self.check_required_args()
        self.jinja_vars['args'] = self.deploy_args # need to update 'args' after applying default values in check_required_args
        self.load_base_conf(k8s_cluster)
        self.load_includes(k8s_cluster)

    def fix_relative_paths(self, y, key, abs_path):
        for x in y:
            if key in x:
                x[key] = os.path.join(abs_path, x[key])

    def build_jinja2_env(self, jinja_vars):
        dirs = []
        for d, _ in self.get_parents():
            dirs.append(d.dir)
        environment = KluctlJinja2Environment(loader=FileSystemLoader(dirs), undefined=StrictUndefined,
                                              cache_size=10000,
                                              bytecode_cache=self.jinja2_cache, auto_reload=False)
        merge_dict(environment.globals, jinja_vars, clone=False)
        add_jinja2_filters(environment)
        return environment

    def load_jinja_vars_list(self, k8s_cluster, vars_list, cur_jinja_vars):
        jinja_vars = copy_dict(cur_jinja_vars)
        for v in vars_list:
            if "values" in v:
                merge_dict(jinja_vars, v["values"], False)
            elif "file" in v:
                jinja_vars = self.load_jinja_vars_file(v["file"], jinja_vars)
            elif "clusterConfigMap" in v:
                ref = ObjectRef(api_version="v1", kind="ConfigMap", name=v["clusterConfigMap"]["name"], namespace=v["clusterConfigMap"]["namespace"])
                jinja_vars = self.load_jinja_vars_k8s_object(k8s_cluster, ref, v["clusterConfigMap"]["key"], jinja_vars)
            elif "clusterSecret" in v:
                ref = ObjectRef(api_version="v1", kind="Secret", name=v["clusterSecret"]["name"], namespace=v["clusterSecret"]["namespace"])
                jinja_vars = self.load_jinja_vars_k8s_object(k8s_cluster, ref, v["clusterSecret"]["key"], jinja_vars)
            else:
                raise CommandError("Invalid vars entry")
        return jinja_vars

    def load_jinja_vars_file(self, rel_file, cur_jinja_vars):
        new_vars = self.load_rendered_yaml(rel_file, cur_jinja_vars)
        jinja_vars = merge_dict(cur_jinja_vars, new_vars)
        return jinja_vars

    def load_jinja_vars_k8s_object(self, k8s_cluster, ref, key, cur_jinja_vars):
        o, _ = k8s_cluster.get_single_object(ref)
        if o is None:
            raise CommandError("%s not found on cluster %s" % (get_long_object_name_from_ref(ref), k8s_cluster.context))
        value = get_dict_value(o, "data.%s" % key)
        if value is None:
            raise CommandError("Key %s not found in %s on cluster %s" % (key, get_long_object_name_from_ref(ref), k8s_cluster.context))
        jinja_env = self.build_jinja2_env(cur_jinja_vars)
        template = jinja_env.from_string(value)
        new_vars = yaml_load(template.render())
        jinja_vars = merge_dict(cur_jinja_vars, new_vars)
        return jinja_vars

    def load_rendered_yaml(self, rel_path, jinja_vars):
        jinja_env = self.build_jinja2_env(jinja_vars)
        template = jinja_env.get_template(rel_path.replace('\\', '/'))
        rendered = template.render()
        return yaml_load(rendered)

    def load_base_conf(self, k8s_cluster):
        base_conf_path = os.path.join(self.dir, 'deployment.yml')
        if not os.path.exists(base_conf_path):
            if os.path.exists(os.path.join(self.dir, 'kustomization.yml')):
                error_text = "deployment.yml not found but folder %s contains kustomization.yml. Is it a kustomizeDir?" % self.dir
            else:
                error_text = "%s not found" % base_conf_path
            raise CommandError(error_text)

        self.conf = self.load_rendered_yaml('deployment.yml', self.jinja_vars)
        if self.conf is None:
            self.conf = {}

        set_dict_default_value(self.conf, 'vars', [])
        set_dict_default_value(self.conf, 'kustomizeDirs', [])
        set_dict_default_value(self.conf, 'includes', [])
        set_dict_default_value(self.conf, 'commonLabels', {})
        set_dict_default_value(self.conf, 'deleteByLabels', {})
        set_dict_default_value(self.conf, 'overrideNamespace', None)
        set_dict_default_value(self.conf, 'tags', [])
        set_dict_default_value(self.conf, 'templateExcludes', [])

        self.jinja_vars = self.load_jinja_vars_list(k8s_cluster, self.conf['vars'], self.jinja_vars)

        for c in self.conf['kustomizeDirs'] + self.conf['includes']:
            if 'tags' not in c and 'path' in c:
                # If there are no explicit tags set, interpret the path as a tag, which allows to
                # enable/disable single deployments via included/excluded tags
                c['tags'] = [os.path.basename(c['path'])]

        if self.get_common_labels() == {} or self.get_common_labels() == {}:
            raise CommandError("No commonLabels/deleteByLabels in root deployment. This is not allowed")

    def check_required_args(self):
        # First try to load the config without templating to avoid getting errors while rendering because required
        # args were not set. Otherwise we won't be able to iterator through the 'args' array in the deployment.yml
        # when the rendering error is actually args related.
        try:
            conf = yaml_load_file(os.path.join(self.dir, 'deployment.yml'))
        except Exception as e:
            # If that failed, it might be that conditional jinja blocks are present in the config, so lets try loading
            # the config in rendered form. If it fails due to missing args now, we can't help much with better error
            # messages anymore.
            conf = self.load_rendered_yaml('deployment.yml', self.jinja_vars)

        if not conf or 'args' not in conf:
            return

        if self.parent_collection is not None:
            raise CommandError("Only the root deployment.yml can contain args")

        self.deploy_args = check_required_args(conf["args"], self.deploy_args)

    def load_includes(self, k8s_cluster):
        for inc in self.conf['includes']:
            if 'path' not in inc:
                continue
            root_project = self.get_root_deployment()

            inc_dir = os.path.join(self.dir, inc["path"])
            inc_dir = os.path.realpath(inc_dir)
            if not inc_dir.startswith(root_project.dir):
                raise CommandError("Include path is not part of root deployment project: %s" % inc["path"])

            c = DeploymentProject(k8s_cluster, inc_dir, self.jinja_vars, self.deploy_args, self.sealed_secrets_dir, parent_collection=self)
            c.parent_collection_include = inc

            inc['_included_deployment_collection'] = c

            self.includes.append(c)

    def get_sealed_secrets_dir(self):
        root = self.get_root_deployment()
        output_pattern = get_dict_value(root.conf, "sealedSecrets.outputPattern")
        return output_pattern

    def get_root_deployment(self):
        if self.parent_collection is None:
            return self
        return self.parent_collection.get_root_deployment()

    def get_parents(self):
        parents = []
        d = self
        inc = None
        while d is not None:
            parents.append((d, inc))
            inc = d.parent_collection_include
            d = d.parent_collection
        return parents

    def get_children(self, recursive, include_self):
        children = []
        if include_self:
            children.append(self)
        for d in self.includes:
            children.append(d)
            if recursive:
                children += d.get_children(True, False)
        return children

    def get_common_labels(self):
        ret = {}
        for d, inc in reversed(self.get_parents()):
            merge_dict(ret, d.conf['commonLabels'], False)
        return ret

    def get_delete_by_labels(self):
        ret = {}
        for d, inc in reversed(self.get_parents()):
            merge_dict(ret, d.conf['deleteByLabels'], False)
        return ret

    def get_override_namespace(self):
        for d, inc in self.get_parents():
            if 'overrideNamespace' in d.conf and d.conf['overrideNamespace'] is not None:
                return d.conf['overrideNamespace']
        return None

    def get_tags(self):
        tags = OrderedSet()
        for d, inc in self.get_parents():
            if inc is not None:
                for t in inc['tags']:
                    tags.add(t)
            for t in d.conf['tags']:
                tags.add(t)
        return tags

    def get_ignore_for_diffs(self, ignore_tags, ignore_labels, ignore_annotations):
        ret = []
        for d, inc in self.get_parents():
            ret += d.conf.get("ignoreForDiff", [])
        if ignore_tags:
            ret.append({
                'fieldPath': 'metadata.labels."kluctl.io/tag-*"',
            })
        if ignore_labels:
            ret.append({
                'fieldPath': 'metadata.labels.*',
            })
        if ignore_annotations:
            ret.append({
                'fieldPath': 'metadata.annotations.*',
            })
        return ret

