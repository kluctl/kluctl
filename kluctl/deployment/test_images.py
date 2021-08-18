import unittest

from kluctl.image_registries.images_registry import ImagesRegistry
from kluctl.deployment.images import Images
from kluctl.utils.k8s_cluster_mocked import k8s_cluster_mocked
from kluctl.utils.versions import PrefixLatestVersion, NumberLatestVersion, LooseSemVerLatestVersion


class TestImages(unittest.TestCase):
    def mocked_k8s_cluster(self):
        k = k8s_cluster_mocked()
        return k

    def build_images_object(self):
        self.registry = MockedImagesRegistry()
        images = Images(self.mocked_k8s_cluster(), [self.registry])
        self.add_images(self.registry)
        return images

    def add_images(self, registry):
        registry.add_tags('g', 'p', '', ['1'])
        registry.add_tags('g', 'p', '', ['2'])
        registry.add_tags('g', 'p', '', ['3.3'])
        registry.add_tags('g', 'p', '', ['a'])
        registry.add_tags('g', 'p', '', ['b'])
        registry.add_tags('g', 'p', '', ['abc'])
        registry.add_tags('g', 'p', '', ['abc-1'])
        registry.add_tags('g', 'p', '', ['z-10'])
        registry.add_tags('g', 'p', '', ['z-1'])
        registry.add_tags('g', 'p2', '', ['1.1'])
        registry.add_tags('g', 'p2', '', ['1.1.1'])
        registry.add_tags('g', 'p3', '', ['1.1'])
        registry.add_tags('g', 'p3', '', ['1.1-abc'])

    def test_gitlab_latest_prefix(self):
        self.assertEqual(self.build_images_object().get_image('registry.gitlab.com/g/p', namespace='ns', deployment_name='d', container='c', latest_version=PrefixLatestVersion('z'))[0], 'registry.gitlab.com/g/p:z-10')

    def test_gitlab_latest_number(self):
        self.assertEqual(self.build_images_object().get_image('registry.gitlab.com/g/p', namespace='ns', deployment_name='d', container='c', latest_version=NumberLatestVersion())[0], 'registry.gitlab.com/g/p:2')

    def test_gitlab_latest_semver_simple(self):
        self.assertEqual(self.build_images_object().get_image('registry.gitlab.com/g/p', namespace='ns', deployment_name='d', container='c', latest_version=LooseSemVerLatestVersion())[0], 'registry.gitlab.com/g/p:3.3')

    def test_gitlab_latest_semver_suffix(self):
        self.assertEqual(self.build_images_object().get_image('registry.gitlab.com/g/p2', namespace='ns', deployment_name='d', container='c', latest_version=LooseSemVerLatestVersion())[0], 'registry.gitlab.com/g/p2:1.1.1')
        self.assertEqual(self.build_images_object().get_image('registry.gitlab.com/g/p3', namespace='ns', deployment_name='d', container='c', latest_version=LooseSemVerLatestVersion())[0], 'registry.gitlab.com/g/p3:1.1')

    def build_images_object_with_deploymet(self, kind):
        images = self.build_images_object()
        images.k8s_cluster.add_object({
            'kind': kind,
            'metadata': {
                'namespace': 'ns',
                'name': 'dep',
            },
            'spec': {
                'template': {
                    'spec': {
                        'containers': [{
                            'name': 'c',
                            'image': 'image-from-deployment:123'
                        }]
                    },
                },
            },
        })
        return images

    def test_deployment(self):
        kind = 'Deployment'
        self.assertEqual(self.build_images_object_with_deploymet(kind).get_image('registry.gitlab.com/g/p', namespace='ns', deployment_name='dep', container='c')[0], 'image-from-deployment:123')
        self.assertEqual(self.build_images_object_with_deploymet(kind).get_image('registry.gitlab.com/g/p', namespace='ns', deployment_name='dep', container='c2')[0], 'registry.gitlab.com/g/p:3.3')
        self.assertEqual(self.build_images_object_with_deploymet(kind).get_image('registry.gitlab.com/g/p', namespace='ns', deployment_name='dep2', container='c')[0], 'registry.gitlab.com/g/p:3.3')

    def test_statefulset(self):
        kind = "StatefulSet"
        self.assertEqual(self.build_images_object_with_deploymet(kind).get_image('registry.gitlab.com/g/p', namespace='ns', deployment_name='StatefulSet/dep', container='c')[0], 'image-from-deployment:123')
        self.assertEqual(self.build_images_object_with_deploymet(kind).get_image('registry.gitlab.com/g/p', namespace='ns', deployment_name='StatefulSet/dep', container='c2')[0], 'registry.gitlab.com/g/p:3.3')
        self.assertEqual(self.build_images_object_with_deploymet(kind).get_image('registry.gitlab.com/g/p', namespace='ns', deployment_name='StatefulSet/dep2', container='c')[0], 'registry.gitlab.com/g/p:3.3')


class MockedImagesRegistry(ImagesRegistry):
    def __init__(self):
        self.images = {}

    def add_tags(self, g, p, r, tags):
        i = f"{g}/{p}"
        if r:
            i += f"/{r}"
        t = self.images.setdefault(i, [])
        t += tags

    def is_image_from_registry(self, image):
        return True

    def list_tags_for_image(self, image):
        image = image.replace("registry.gitlab.com/", "")
        return self.images.get(image)
