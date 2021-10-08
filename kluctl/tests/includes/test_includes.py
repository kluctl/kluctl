import os

from kluctl.deployment.deployment_collection import DeploymentCollection
from kluctl.tests.test_base import DeploymentTestBase

cur_dir = os.path.dirname(__file__)

class TestIncludes(DeploymentTestBase):
    def build_fixed_deployment(self):
        return self.build_deployment(os.path.join("includes", "test_deployment"))

    def test_simple(self):
        self.assertEqual(len(self.d.includes), 2)
        self.assertEqual(len(self.d.conf['kustomizeDirs']), 0)
        self.assertEqual(len(self.d.includes[0].conf['kustomizeDirs']), 1)
        self.assertEqual(len(self.d.includes[1].conf['kustomizeDirs']), 1)
        self.assertEqual(len(self.d.includes[0].includes), 1)
        self.assertEqual(len(self.d.includes[1].includes), 1)

    def test_deployment_pathes(self):
        self.assertEqual(self.d.includes[0].dir, os.path.join(cur_dir, 'test_deployment', 'd1'))
        self.assertEqual(self.d.includes[1].dir, os.path.join(cur_dir, 'test_deployment', 'd2'))

    def test_common_label_merge(self):
        self.assertDictEqual(self.d.get_common_labels(), {'a': 'a1', 'b': 'b1'})
        self.assertDictEqual(self.d.includes[0].get_common_labels(), {'a': 'a2', 'b': 'b1', 'c': 'c1'})

    def test_common_label_merge_sub_include(self):
        self.assertDictEqual(self.d.includes[0].includes[0].get_common_labels(), {'a': 'a2', 'b': 'b1', 'c': 'c1'})

    def test_delete_by_label_merge(self):
        self.assertDictEqual(self.d.get_delete_by_labels(), {'da': 'a1', 'db': 'b1'})
        self.assertDictEqual(self.d.includes[0].get_delete_by_labels(), {'da': 'a2', 'db': 'b1', 'dc': 'c1'})

    def test_delete_by_label_merge_sub_include(self):
        self.assertDictEqual(self.d.includes[0].includes[0].get_delete_by_labels(), {'da': 'a2', 'db': 'b1', 'dc': 'c1'})

    def test_override_namespace(self):
        self.assertEqual(self.d.get_override_namespace(), 'o1')
        self.assertEqual(self.d.includes[0].get_override_namespace(), 'o2')

    def test_override_namespace_sub_include(self):
        self.assertEqual(self.d.includes[1].includes[0].get_override_namespace(), 'o1')

    def test_collection_tags(self):
        self.assertSetEqual(self.d.get_tags(), {'t1', 't2'})
        self.assertSetEqual(self.d.includes[0].get_tags(), {'t1', 't2', 'd1'}) # d1 is equal to the include name
        self.assertSetEqual(self.d.includes[1].get_tags(), {'t1', 't2', 'it1', 'it2'})

    def test_collection_tags_sub_include(self):
        self.assertSetEqual(self.d.includes[0].includes[0].get_tags(), {'t1', 't2', 'd1', 'd1_sub'})

    def test_get_kustomize_dir_tags(self):
        self.assertSetEqual(set(DeploymentCollection(self.d.includes[0], None, None, None, False).deployments[0].get_tags()), {'t1', 't2', 'd1', 'k1'})
        self.assertSetEqual(set(DeploymentCollection(self.d.includes[1], None, None, None, False).deployments[0].get_tags()), {'t1', 't2', 'it1', 'it2', 'k1'})
