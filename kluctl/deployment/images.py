import hashlib
import logging
import threading

from kluctl.utils.dict_utils import copy_dict, set_dict_value, get_dict_value, object_iterator
from kluctl.utils.exceptions import CommandError
from kluctl.utils.thread_safe_cache import ThreadSafeMultiCache
from kluctl.utils.versions import build_latest_version_from_str
from kluctl.utils.yaml_utils import yaml_dump

logger = logging.getLogger(__name__)


class Images(object):
    def __init__(self, image_registries):
        self.image_registries = image_registries
        self.update_images = False
        self.raise_on_error = False
        self.image_tags_cache = ThreadSafeMultiCache()

        self.placeholders = {}
        self.seen_images = []
        self.seen_images_lock = threading.Lock()
        self.fixed_images = []

    def get_registry_for_image(self, image):
        for r in self.image_registries:
            if r.is_image_from_registry(image):
                return r
        raise Exception('Registry for image %s is not supported or not initialized (missing credentials?)' % image)

    def get_image_tags_from_registry(self, image):
        def do_get():
            r = self.get_registry_for_image(image)
            tags = r.list_tags_for_image(image)
            return tags
        return self.image_tags_cache.get(image, "tags", do_get)

    def get_latest_image_from_registry(self, image, latest_version):
        if isinstance(latest_version, str):
            latest_version = build_latest_version_from_str(latest_version)

        tags = self.get_image_tags_from_registry(image)
        filtered_tags = latest_version.filter(tags)
        if len(filtered_tags) == 0:
            logger.error(f"Failed to find latest image for {image}. Available tags are: {','.join(tags)}")
            return None

        latest_tag = latest_version.latest(filtered_tags)

        logger.debug(f"get_latest_image_from_registry({image}): returned {latest_tag}")

        return f'{image}:{latest_tag}'

    def build_seen_entry(self, image, latest_version,
                         result_image, deployed_image, registry_image,
                         namespace, deployment, container):
        new_entry = {
            "image": image,
        }
        if latest_version is not None:
            new_entry["versionFilter"] = latest_version
        if result_image is not None:
            new_entry["resultImage"] = result_image
        if deployed_image is not None:
            new_entry["deployedImage"] = deployed_image
        if registry_image is not None:
            new_entry["registryImage"] = registry_image

        if namespace is not None:
            new_entry["namespace"] = namespace
        if deployment is not None:
            new_entry["deployment"] = deployment
        if container is not None:
            new_entry["container"] = container

        return new_entry

    def add_seen_image(self, image, latest_version,
                       result_image, deployed_image, registry_image,
                       kustomize_dir, tags,
                       namespace, deployment, container):
        new_entry = self.build_seen_entry(image, latest_version, result_image, deployed_image, registry_image,
                                          namespace, deployment, container)
        new_entry["deployTags"] = tags
        new_entry["kustomizeDir"] = kustomize_dir

        with self.seen_images_lock:
            if new_entry not in self.seen_images:
                self.seen_images.append(new_entry)

    def add_fixed_image(self, fixed_image):
        if "image" not in fixed_image:
            raise CommandError("'image' is missing in fixed image entry")
        if "resultImage" not in fixed_image:
            raise CommandError("'resultImage' is missing in fixed image entry")

        e = {
            "image": fixed_image["image"],
            "resultImage": fixed_image["resultImage"],
        }
        if "namespace" in fixed_image:
            e["namespace"] = fixed_image["namespace"]
        if "deployment" in fixed_image:
            e["deployment"] = fixed_image["deployment"]
        if "container" in fixed_image:
            e["container"] = fixed_image["container"]

        # put it to the front so that newest entries are preferred
        self.fixed_images.insert(0, e)

    def get_fixed_image(self, image, namespace, deployment, container):
        e = self.build_seen_entry(image, None, None, None, None, namespace, deployment, container)
        for fi in self.fixed_images:
            if fi["image"] != image:
                continue
            match = True
            for k, v in fi.items():
                if k == "resultImage":
                    continue
                if v != e.get(k):
                    match = False
                    break
            if match:
                return fi["resultImage"]
        return None

    def gen_image_placeholder(self, image, latest_version, kustomize_dir, tags):
        placeholder = {
            "image": image,
            "latest_version": str(latest_version),
            "kustomize_dir": kustomize_dir,
            "tags": tags,
        }
        h = hashlib.sha256(yaml_dump(placeholder).encode("utf-8")).hexdigest()
        with self.seen_images_lock:
            self.placeholders[h] = placeholder
        return h

    def resolve_placeholders(self, local_object, remote_object):
        for v, p in object_iterator(copy_dict(local_object)):
            if not isinstance(v, str):
                continue
            placeholder = self.placeholders.get(v)
            if placeholder is None:
                continue

            namespace = get_dict_value(local_object, "metadata.namespace")
            deployment = "%s/%s" % (local_object["kind"], get_dict_value(local_object, "metadata.name"))
            container = get_dict_value(local_object, p[:-1] + ["name"])

            fixed = self.get_fixed_image(placeholder["image"], namespace, deployment, container)
            deployed = None
            if remote_object is not None:
                deployed = get_dict_value(remote_object, p)
            registry = self.get_latest_image_from_registry(placeholder["image"], placeholder["latest_version"])

            result = deployed
            if result is None or self.update_images:
                result = registry
            if not self.update_images and fixed is not None:
                result = fixed

            self.add_seen_image(placeholder["image"], placeholder["latest_version"], result, deployed, registry,
                                placeholder["kustomize_dir"], placeholder["tags"],
                                namespace, deployment, container)

            set_dict_value(local_object, p, result)
