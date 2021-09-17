import unittest

from kluctl.deployment.images import Images
from kluctl.image_registries.images_registry import ImagesRegistry
from kluctl.utils.dict_utils import get_dict_value
from kluctl.utils.versions import PrefixLatestVersion, NumberLatestVersion, LooseSemVerLatestVersion


class TestImages(unittest.TestCase):
    def build_images_object(self):
        self.registry = MockedImagesRegistry()
        images = Images([self.registry])
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
        self.assertEqual(self.build_images_object().get_latest_image_from_registry('registry.gitlab.com/g/p', latest_version=PrefixLatestVersion('z')), 'registry.gitlab.com/g/p:z-10')

    def test_gitlab_latest_number(self):
        self.assertEqual(self.build_images_object().get_latest_image_from_registry('registry.gitlab.com/g/p', latest_version=NumberLatestVersion()), 'registry.gitlab.com/g/p:2')

    def test_gitlab_latest_semver_simple(self):
        self.assertEqual(self.build_images_object().get_latest_image_from_registry('registry.gitlab.com/g/p', latest_version=LooseSemVerLatestVersion()), 'registry.gitlab.com/g/p:3.3')

    def test_gitlab_latest_semver_suffix(self):
        self.assertEqual(self.build_images_object().get_latest_image_from_registry('registry.gitlab.com/g/p2', latest_version=LooseSemVerLatestVersion()), 'registry.gitlab.com/g/p2:1.1.1')
        self.assertEqual(self.build_images_object().get_latest_image_from_registry('registry.gitlab.com/g/p3', latest_version=LooseSemVerLatestVersion()), 'registry.gitlab.com/g/p3:1.1')

    def build_object(self, image1, image2):
        o = {
            'kind': 'Deployment',
            'metadata': {
                'namespace': 'ns',
                'name': 'dep',
            },
            'spec': {
                'template': {
                    'spec': {
                        'containers': []
                    },
                },
            },
        }
        if image1 is not None:
            o["spec"]["template"]["spec"]["containers"].append({
                "name": "c1",
                "image": image1
            })
        if image2 is not None:
            o["spec"]["template"]["spec"]["containers"].append({
                "name": "c2",
                "image": image2
            })
        return o

    def do_test_placeholder_resolve(self, image, deployed_object, result_image1, result_image2):
        images = self.build_images_object()
        latest_version = LooseSemVerLatestVersion()
        placeholder = images.gen_image_placeholder(image, latest_version, None, [])
        rendered_object = self.build_object(placeholder, placeholder)
        images.resolve_placeholders(rendered_object, deployed_object)
        self.assertEqual(get_dict_value(rendered_object, "spec.template.spec.containers[0]image"), result_image1)
        self.assertEqual(get_dict_value(rendered_object, "spec.template.spec.containers[1]image"), result_image2)

    def test_deployed(self):
        self.do_test_placeholder_resolve('registry.gitlab.com/g/p', self.build_object("di1", "di2"), "di1", "di2")

    def test_not_deployed(self):
        self.do_test_placeholder_resolve('registry.gitlab.com/g/p', None, "registry.gitlab.com/g/p:3.3", "registry.gitlab.com/g/p:3.3")

    def test_container_missing(self):
        self.do_test_placeholder_resolve('registry.gitlab.com/g/p', self.build_object("di1", None), "di1", "registry.gitlab.com/g/p:3.3")


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
