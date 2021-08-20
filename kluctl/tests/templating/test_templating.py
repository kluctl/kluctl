import os

from kluctl.tests.test_base import DeploymentTestBase
from kluctl.utils.yaml_utils import yaml_load_file

cur_dir = os.path.dirname(__file__)

class TestTemplating(DeploymentTestBase):
    def get_jinja2_vars(self):
        return {
            'a': 'a1',
            'b': 'b1',
            'include_var': 'd1',
        }

    def test_deployment_yml(self):
        with self.build_deployment('templating/test_deployment', self.get_jinja2_vars(), {'a': 'a2'}) as (d, c):
            self.assertEqual(len(d.includes), 2)
            self.assertListEqual(d.conf['tags'], ['a1', 'a2'])

    def test_include_var(self):
        with self.build_deployment('templating/test_deployment', self.get_jinja2_vars(), {'a': 'a2'}) as (d, c):
            self.assertEqual(d.includes[0].dir, os.path.join(cur_dir, 'test_deployment', 'd1'))

    def test_not_rendered_kustomize_resource(self):
        with self.render_deployment('templating/test_deployment', self.get_jinja2_vars(), {'a': 'a2'}) as c:
            y = yaml_load_file(os.path.join(c.tmpdir, 'd1/k1/not-rendered.yml'))
            self.assertEqual(y['a'], '{{ a }}')

    def test_rendered_kustomize_resource(self):
        with self.render_deployment('templating/test_deployment', self.get_jinja2_vars(), {'a': 'a2'}) as c:
            y = yaml_load_file(os.path.join(c.tmpdir, 'd1/k1/rendered.yml'))
            self.assertEqual(y['a'], 'a1')

    def test_load_template(self):
        with self.render_deployment('templating/test_deployment', self.get_jinja2_vars(), {'a': 'a2'}) as c:
            y = yaml_load_file(os.path.join(c.tmpdir, 'd1/k1/rendered.yml'))
            self.assertEqual(y['b'], 'test a1')
            self.assertEqual(y['c'], 'test a1')

    def test_rendered_kustomization_yml(self):
        with self.render_deployment('templating/test_deployment', self.get_jinja2_vars(), {'a': 'a2'}) as c:
            y = yaml_load_file(os.path.join(c.tmpdir, 'd1/k1/kustomization.yml'))
            self.assertListEqual(y['resources'], ['b1'])

    def test_import_no_context(self):
        with self.render_deployment('templating/test_import', self.get_jinja2_vars(), {}) as c:
            y = yaml_load_file(os.path.join(c.tmpdir, 'k1/rendered.yml'))
            self.assertEqual(y['a'], 'a1')

    def test_get_var(self):
        with self.render_deployment('templating/test_utils', self.get_jinja2_vars(), {}) as c:
            y = yaml_load_file(os.path.join(c.tmpdir, 'k1/get_var.yml'))
            self.assertEqual(y['test1'], 'default')
            self.assertEqual(y['test2'], 'default')
            self.assertEqual(y['test3'], 'a')

    def test_vars(self):
        with self.render_deployment('templating/test_vars', self.get_jinja2_vars(), {}) as c:
            y = yaml_load_file(os.path.join(c.tmpdir, 'k1/test.yml'))
            self.assertEqual(y['test1'], 'v1')
            self.assertEqual(y['test2'], 'f1')
            self.assertEqual(y['test3'], 'v1')
            self.assertEqual(y['test4'], 'b')
