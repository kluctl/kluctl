import logging
import threading

from kluctl.command_error import CommandError
from kluctl.utils.thread_safe_cache import ThreadSafeMultiCache
from kluctl.utils.versions import LooseSemVerLatestVersion

logger = logging.getLogger(__name__)


class Images(object):
    def __init__(self, k8s_cluster, image_registries):
        self.k8s_cluster = k8s_cluster
        self.image_registries = image_registries
        self.update_images = False
        self.no_registries = False
        self.no_kubernetes = False
        self.raise_on_error = False
        self.image_tags_cache = ThreadSafeMultiCache()
        self.k8s_object_cache = ThreadSafeMultiCache()

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
        if self.no_registries:
            return None

        tags = self.get_image_tags_from_registry(image)
        filtered_tags = latest_version.filter(tags)
        if len(filtered_tags) == 0:
            logger.error(f"Failed to find latest image for {image}. Available tags are: {','.join(tags)}")
            return None

        latest_tag = latest_version.latest(filtered_tags)

        logger.debug(f"get_latest_image_from_registry({image}): returned {latest_tag}")

        return f'{image}:{latest_tag}'

    def get_image_from_k8s_object(self, namespace, name_with_kind, container):
        if self.no_kubernetes:
            return None

        kind = 'Deployment'
        name = name_with_kind
        a = name_with_kind.split('/')
        if len(a) != 1:
            kind = a[0]
            name = a[1]

        def get_object():
            l = self.k8s_cluster.get_objects(kind=kind, name=name, namespace=namespace)
            if not l:
                return None
            if len(l) != 1:
                raise Exception("expected single object, got %d" % len(l))
            return l[0]

        ret = None

        r = self.k8s_object_cache.get((kind, name, namespace), "object", get_object)
        if r is not None:
            deployment = r[0]
            for c in deployment['spec']['template']['spec']['containers']:
                if c['name'] == container:
                    ret = c['image']
                    break

        logger.debug(f"get_image_from_k8s_object({namespace}, {name_with_kind}, {container}): returned {ret}")
        return ret

    def get_image(self, image, namespace=None, deployment_name=None, container=None, latest_version=LooseSemVerLatestVersion()):
        registry_image = self.get_latest_image_from_registry(image, latest_version)
        deployed_image = None
        if namespace is not None and deployment_name is not None and container is not None:
            deployed_image = self.get_image_from_k8s_object(namespace, deployment_name, container)

        result_image = deployed_image
        if result_image is None or self.update_images:
            result_image = registry_image

        if result_image is None and self.raise_on_error:
            raise Exception("Failed to determine image for %s" % image)
        return result_image, deployed_image, registry_image

class SeenImages:
    def __init__(self, images):
        self.images = images
        self.seen_images = []
        self.seen_images_lock = threading.Lock()
        self.fixed_images = []

    def build_seen_entry(self, image, result_image, deployed_image, registry_image, kwargs):
        new_entry = {
            "image": image,
        }
        if result_image is not None:
            new_entry["resultImage"] = result_image
        if deployed_image is not None:
            new_entry["deployedImage"] = deployed_image
        if registry_image is not None:
            new_entry["registryImage"] = registry_image

        def set_value(kw_name, entry_name):
            v = kwargs.get(kw_name)
            if v is not None:
                new_entry[entry_name] = str(v)

        set_value("namespace", "namespace")
        set_value("deployment_name", "deployment")
        set_value("container", "container")
        set_value("latest_version", "versionFilter")
        return new_entry

    def add_seen_image(self, image, result_image, deployed_image, registry_image, kustomize_dir, tags, kwargs):
        new_entry = self.build_seen_entry(image, result_image, deployed_image, registry_image, kwargs)
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

    def get_fixed_image(self, image, kwargs):
        e = self.build_seen_entry(image, None, None, None, kwargs)
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

    def get_image_wrapper(self, kustomize_dir, tags):
        def wrapper(image, **kwargs):
            return self.do_get_image(kustomize_dir, tags, image, kwargs)
        return wrapper

    def do_get_image(self, kustomize_dir, tags, image, kwargs):
        fixed = self.get_fixed_image(image, kwargs)
        result, deployed, registry = self.images.get_image(image, **kwargs)
        if not self.images.update_images and fixed is not None:
            result = fixed
        self.add_seen_image(image, result, deployed, registry, kustomize_dir, tags, kwargs)
        return result
