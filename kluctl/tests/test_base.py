import contextlib
import os
import tempfile
import unittest

from kluctl.deployment.deployment_collection import DeploymentCollection
from kluctl.deployment.deployment_project import DeploymentProject
from kluctl.utils.utils import get_tmp_base_dir

cur_dir = os.path.dirname(__file__)

class DeploymentTestBase(unittest.TestCase):

    def __init__(self, *args, **kwargs):
        super().__init__(*args, **kwargs)
        self.context = None
        self.d = None
        self.c = None

    def build_fixed_deployment(self):
        return None

    def setUp(self):
        self.context = self.build_fixed_deployment()
        if self.context is None:
            return
        self.d, self.c = self.context.__enter__()

    def tearDown(self):
        if self.context is None:
            return
        try:
            self.context.__exit__(None, None, None)
        finally:
            self.context = None
            self.d = None
            self.c = None

    @contextlib.contextmanager
    def build_deployment(self, subdir, jinja_vars={}, deploy_args={}):
        with tempfile.TemporaryDirectory(dir=get_tmp_base_dir()) as tmpdir:
            d = DeploymentProject(os.path.join(cur_dir, subdir), jinja_vars, deploy_args,
                                  os.path.join(cur_dir, subdir, '.sealed-secrets'))
            c = DeploymentCollection(d, None, None, tmpdir, False)
            yield d, c

    @contextlib.contextmanager
    def render_deployment(self, subdir, jinja_vars={}, deploy_args={}):
        with self.build_deployment(subdir, jinja_vars, deploy_args) as (d, c):
            c.render_deployments()
            yield c
