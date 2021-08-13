from kluctl.tests.test_base import DeploymentTestBase

class TestTemplating(DeploymentTestBase):
    def test_stable_tags_ordering(self):
        with self.build_deployment("misc/test_deployment") as (d, c):
            self.assertListEqual(list(d.get_tags()), ['a', 'c', 'b', 'g', '1'])
