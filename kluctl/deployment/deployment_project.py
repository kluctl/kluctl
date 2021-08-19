import logging
import os

from jinja2 import FileSystemLoader, StrictUndefined, FileSystemBytecodeCache
from ordered_set import OrderedSet

from kluctl.utils.exceptions import CommandError
from kluctl.utils.external_args import check_required_args
from kluctl.utils.jinja2_utils import add_jinja2_filters, RelEnvironment, render_str
from kluctl.utils.utils import merge_dict, copy_dict, \
    set_default_value
from kluctl.utils.yaml_utils import yaml_load, yaml_load_file

logger = logging.getLogger(__name__)


class DeploymentProject(object):
    def __init__(self, dir, deployment_name, jinja_vars, deploy_args, sealed_secrets_dir,
                 parent_collection=None):
        if not os.path.exists(dir):
            raise CommandError("%s does not exist" % dir)

        self.dir = dir
        self.deployment_name = deployment_name
        if self.deployment_name is None:
            self.deployment_name = os.path.basename(self.dir)
        self.jinja_vars = copy_dict(jinja_vars)
        self.deploy_args = deploy_args
        self.sealed_secrets_dir = sealed_secrets_dir
        self.jinja_vars['args'] = self.deploy_args
        self.includes = []
        self.parent_collection = parent_collection
        self.parent_collection_include = None
        self.jinja_vars = self.load_jinja_vars('./jinja2-vars.yml')
        self.check_required_args()
        self.jinja_vars['args'] = self.deploy_args # need to update 'args' after applying default values in check_required_args
        self.load_base_conf()
        self.load_includes()

    def fix_relative_paths(self, y, key, abs_path):
        for x in y:
            if key in x:
                x[key] = os.path.join(abs_path, x[key])

    def build_jinja2_env(self, jinja_vars):
        dirs = []
        for d, _ in self.get_parents():
            dirs.append(d.dir)
        cache = FileSystemBytecodeCache()
        environment = RelEnvironment(loader=FileSystemLoader(dirs), undefined=StrictUndefined,
                                     cache_size=10000,
                                     bytecode_cache=cache, auto_reload=False)
        merge_dict(environment.globals, jinja_vars, clone=False)
        add_jinja2_filters(environment)
        return environment

    def load_jinja_vars(self, rel_file):
        if not os.path.exists(os.path.join(self.dir, rel_file)):
            return self.jinja_vars
        new_vars = self.load_rendered_yaml(rel_file, self.jinja_vars)
        jinja_vars = merge_dict(self.jinja_vars, new_vars)
        return jinja_vars

    def load_rendered_yaml(self, rel_path, jinja_vars):
        jinja_env = self.build_jinja2_env(jinja_vars)
        template = jinja_env.get_template(rel_path.replace('\\', '/'))
        rendered = template.render()
        return yaml_load(rendered)

    def load_base_conf(self):
        if not os.path.exists(os.path.join(self.dir, 'deployment.yml')):
            raise CommandError("deployment.yml not found")

        self.conf = self.load_rendered_yaml('deployment.yml', self.jinja_vars)
        if self.conf is None:
            self.conf = {}

        set_default_value(self.conf, 'vars', [])
        set_default_value(self.conf, 'kustomizeDirs', [])
        set_default_value(self.conf, 'includes', [])
        set_default_value(self.conf, 'commonLabels', {})
        set_default_value(self.conf, 'deleteByLabels', {})
        set_default_value(self.conf, 'overrideNamespace', None)
        set_default_value(self.conf, 'tags', [])
        set_default_value(self.conf, 'templateExcludes', [])

        for v in self.conf['vars']:
            if 'values' in v:
                merge_dict(self.jinja_vars, v['values'], False)
            elif 'file' in v:
                self.jinja_vars = self.load_jinja_vars(v['file'])

        for c in self.conf['kustomizeDirs'] + self.conf['includes']:
            if 'tags' not in c and 'path' in c:
                # If there are no explicit tags set, interpret the path as a tag, which allows to
                # enable/disable single deployments via included/excluded tags
                c['tags'] = [os.path.basename(c['path'])]

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

    def load_includes(self):
        for inc in self.conf['includes']:
            if 'path' not in inc:
                continue
            inc_dir = os.path.join(self.dir, inc["path"])
            c = DeploymentProject(inc_dir, None, self.jinja_vars, self.deploy_args, self.sealed_secrets_dir, parent_collection=self)
            c.parent_collection_include = inc

            inc['_included_deployment_collection'] = c

            self.includes.append(c)

    def get_sealed_secrets_dir(self, subdir):
        root = self.get_root_deployment()
        output_pattern = root.conf.get("secrets", {}).get("outputPattern")
        return output_pattern

    def get_root_deployment(self):
        if self.parent_collection is None:
            return self
        return self.parent_collection.get_root_deployment()

    def get_rel_dir_to_root(self, subdir=None):
        root = self.get_root_deployment()
        root_dir = os.path.abspath(root.dir)
        dir = self.dir
        if subdir is not None:
            dir = os.path.join(dir, subdir)
        dir = os.path.abspath(dir)
        rel_dir = os.path.relpath(dir, root_dir)
        return rel_dir

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

    def get_ignore_for_diffs(self, ignore_tags, ignore_labels):
        ret = []
        for d, inc in self.get_parents():
            ret += d.conf.get("ignoreForDiff", [])
        if ignore_tags:
            ret.append({
                'fieldPath': 'metadata.labels.kluctl.io/tag-*',
            })
        if ignore_labels:
            ret.append({
                'fieldPath': 'metadata.labels.*',
            })
        return ret

