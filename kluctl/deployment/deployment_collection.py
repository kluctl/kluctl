import dataclasses
import functools
import itertools
import logging
import os

from kubernetes.dynamic.exceptions import ResourceNotFoundError, ConflictError

from kluctl.deployment.apply_util import ApplyUtil
from kluctl.deployment.kustomize_deployment import KustomizeDeployment
from kluctl.diff.k8s_diff import deep_diff_object
from kluctl.diff.normalize import normalize_object
from kluctl.utils.dict_utils import copy_dict, get_dict_value
from kluctl.utils.k8s_delete_utils import find_objects_for_delete
from kluctl.utils.k8s_downscale_utils import downscale_object
from kluctl.utils.k8s_object_utils import get_object_ref, get_long_object_name, get_long_object_name_from_ref, \
    ObjectRef
from kluctl.utils.k8s_status_validation import validate_object, ValidateResult, ValidateResultItem
from kluctl.utils.templated_dir import TemplatedDir
from kluctl.utils.utils import MyThreadPoolExecutor

logger = logging.getLogger(__name__)

@dataclasses.dataclass(frozen=True, eq=True)
class DeployErrorItem:
    ref: ObjectRef
    message: str

@dataclasses.dataclass
class DeployDiffResult:
    new_objects: list
    changed_objects: list
    errors: list
    warnings: list

class DeploymentCollection:
    def __init__(self, project, images, inclusion, tmpdir, for_seal):
        self.project = project
        self.images = images
        self.inclusion = inclusion
        self.tmpdir = tmpdir
        self.for_seal = for_seal
        self.deployments = self._collect_deployments(self.project)

        self.remote_objects = {}

        self.errors = set()
        self.warnings = set()

    def _collect_deployments(self, project):
        ret = []

        indexes = {}
        for c in project.conf['kustomizeDirs']:
            index = 0
            if "path" in c:
                p = os.path.normpath(c["path"])
                index = indexes.setdefault(p, 0)
                indexes[p] += 1
            deployment = KustomizeDeployment(project, self, c, index)
            ret.append(deployment)

        for inc in project.conf['includes']:
            if get_dict_value(inc, "barrier", False):
                deployment = KustomizeDeployment(project, self, {"barrier": True}, 0)
                ret.append(deployment)

            d = inc.get('_included_deployment_collection')
            if d is not None:
                ret += self._collect_deployments(d)

        return ret

    def render_deployments(self):
        logger.info("Rendering templates and Helm charts")
        with MyThreadPoolExecutor(max_workers=16) as executor:
            jobs = []

            for d in self.deployments:
                jobs += d.render(executor)

            TemplatedDir.finish_jobs(jobs)

            jobs = []
            for d in self.deployments:
                jobs += d.render_helm_charts(executor)
            for job in jobs:
                job.result()

    def resolve_sealed_secrets(self):
        if self.for_seal:
            return

        for d in self.deployments:
            d.resolve_sealed_secrets()

    def build_kustomize_objects(self, k8s_cluster):
        logger.info("Building kustomize objects")

        with MyThreadPoolExecutor() as executor:
            jobs = []
            for d in self.deployments:
                jobs.append(executor.submit(d.build_kustomize))
            for job in jobs:
                job.result()
            jobs.clear()

            for d in self.deployments:
                jobs.append(executor.submit(d.postprocess_and_load_objects, k8s_cluster))
            for job in jobs:
                job.result()

    def update_remote_objects(self, k8s_cluster):
        if k8s_cluster is None:
            return

        logger.info("Updating remote objects")
        refs = set()
        for ref in self.local_objects_by_ref().keys():
            if ref not in self.remote_objects:
                refs.add(ref)
        r = k8s_cluster.get_objects_by_object_refs(refs)
        for o, w in r:
            self.remote_objects[get_object_ref(o)] = o
            self.add_warnings(get_object_ref(o), w)

    def forget_remote_objects(self, refs):
        for ref in refs:
            self.remote_objects.pop(ref, None)

    def local_objects_by_ref(self):
        flat = list(itertools.chain(*[d.objects for d in self.deployments]))
        by_ref = dict((get_object_ref(x), x) for x in flat)
        return by_ref

    def prepare(self, k8s_cluster):
        self.render_deployments()
        self.resolve_sealed_secrets()
        self.build_kustomize_objects(k8s_cluster)
        self.update_remote_objects(k8s_cluster)

    def deploy(self, k8s_cluster, force_apply, replace_on_error, force_replace_on_error, abort_on_error):
        self.clear_errors_and_warnings()
        applied_objects = self.do_apply(k8s_cluster, force_apply, replace_on_error, force_replace_on_error,
                                        False, abort_on_error)
        new_objects, changed_objects = self.do_diff(k8s_cluster, applied_objects, False, False, False, False)
        return DeployDiffResult(new_objects=new_objects, changed_objects=changed_objects,
                                errors=list(self.errors), warnings=list(self.warnings))

    def diff(self, k8s_cluster, force_apply, replace_on_error, force_replace_on_error, ignore_tags, ignore_labels, ignore_annotations, ignore_order):
        self.clear_errors_and_warnings()
        applied_objects = self.do_apply(k8s_cluster, force_apply, replace_on_error, force_replace_on_error,
                                        True, False)
        new_objects, changed_objects = self.do_diff(k8s_cluster, applied_objects, ignore_tags, ignore_labels, ignore_annotations, ignore_order)
        return DeployDiffResult(new_objects=new_objects, changed_objects=changed_objects,
                                errors=list(self.errors), warnings=list(self.warnings))

    def poke_images(self, k8s_cluster):
        self.clear_errors_and_warnings()
        def do_poke_image(containers_and_images, o):
            o = copy_dict(o)
            containers = get_dict_value(o, "spec.template.spec.containers", [])
            for container_name, image in containers_and_images:
                for c in containers:
                    if c["name"] == container_name:
                        c["image"] = image
            return o

        all_objects = {}
        for d in self.deployments:
            if not d.check_inclusion_for_deploy():
                continue
            for o in d.objects:
                all_objects[get_object_ref(o)] = o

        containers_and_images = {}
        for fi in self.images.seen_images:
            deployment_name = fi["deployment"]
            if "/" not in deployment_name:
                deployment_name = "Deployment/%s" % deployment_name

            kind, name = tuple(deployment_name.split("/", 1))
            try:
                r = k8s_cluster.get_preferred_resource(None, kind)
            except ResourceNotFoundError as e:
                self.add_error(None, k8s_cluster.get_status_message(e))
                continue

            ref = ObjectRef(r.group_version, kind=r.kind, name=name, namespace=fi.get("namespace"))
            local_object = all_objects.get(ref)
            if local_object is None:
                self.add_error(ref, "object not found while trying to associate image with deployed object")
                continue

            containers_and_images.setdefault(ref, []).append((fi["container"], fi["resultImage"]))

        with MyThreadPoolExecutor(max_workers=8) as executor:
            futures = []

            for ref, c in containers_and_images.items():
                f = executor.submit(self.do_replace_object, k8s_cluster, ref, functools.partial(do_poke_image, c))
                futures.append((ref, f))

            applied_objects = self.do_finish_futures(futures)

        new_objects, changed_objects = self.do_diff(k8s_cluster, applied_objects, False, False, False, False)
        return DeployDiffResult(new_objects=new_objects, changed_objects=changed_objects,
                                errors=list(self.errors), warnings=list(self.warnings))

    def downscale(self, k8s_cluster):
        self.clear_errors_and_warnings()
        with MyThreadPoolExecutor(max_workers=8) as executor:
            futures = []
            for d in self.deployments:
                if not d.check_inclusion_for_deploy():
                    continue
                for o in d.objects:
                    ref = get_object_ref(o)
                    f = executor.submit(self.do_replace_object, k8s_cluster, ref, downscale_object)
                    futures.append((ref, f))

            applied_objects = self.do_finish_futures(futures)

        new_objects, changed_objects = self.do_diff(k8s_cluster, applied_objects, False, False, False, False)
        return DeployDiffResult(new_objects=new_objects, changed_objects=changed_objects,
                                errors=list(self.errors), warnings=list(self.warnings))

    def validate(self):
        self.clear_errors_and_warnings()
        result = ValidateResult()

        for w in self.warnings:
            result.warnings.append(ValidateResultItem(ref=w.ref, reason="api", message=w.message))
        for e in self.errors:
            result.errors.append(ValidateResultItem(ref=e.ref, reason="api", message=e.message))

        for d in self.deployments:
            if not d.check_inclusion_for_deploy():
                continue
            for o in d.objects:
                ref = get_object_ref(o)
                remote_object = self.remote_objects.get(ref)
                if not remote_object:
                    result.errors.append(ValidateResultItem(ref=ref, reason="not-found", message="Object not found"))
                    continue
                r = validate_object(remote_object, True)
                result.errors += r.errors
                result.warnings += r.warnings
                result.results += r.results
        return result

    def find_delete_objects(self, k8s_cluster):
        self.clear_errors_and_warnings()
        labels = self.project.get_delete_by_labels()
        return find_objects_for_delete(k8s_cluster, labels, self.inclusion, [])

    def find_prune_objects(self, k8s_cluster):
        self.clear_errors_and_warnings()
        logger.info("Searching objects not found in local objects")
        labels = self.project.get_delete_by_labels()
        excluded_objects = list(self.local_objects_by_ref().keys())
        return find_objects_for_delete(k8s_cluster, labels, self.inclusion, excluded_objects)

    def do_apply(self, k8s_cluster, force_apply, replace_on_error, force_replace_on_error, dry_run, abort_on_error):
        if k8s_cluster.dry_run:
            dry_run = dry_run
        apply_util = ApplyUtil(self, k8s_cluster, force_apply, replace_on_error, force_replace_on_error, dry_run, abort_on_error)
        apply_util.apply_deployments()
        return apply_util.applied_objects

    def do_diff(self, k8s_cluster, applied_objects, ignore_tags, ignore_labels, ignore_annotations, ignore_order):
        diff_objects = {}
        normalized_diff_objects = {}
        normalized_remote_objects = {}
        for d in self.deployments:
            if not d.check_inclusion_for_deploy():
                continue

            ignore_for_diffs = d.deployment_project.get_ignore_for_diffs(ignore_tags, ignore_labels, ignore_annotations)
            for x in d.objects:
                ref = get_object_ref(x)
                if ref not in applied_objects:
                    continue
                diff_objects[ref] = applied_objects.get(ref)
                normalized_diff_objects[ref] = normalize_object(k8s_cluster, diff_objects[ref], ignore_for_diffs)
                if ref in self.remote_objects:
                    normalized_remote_objects[ref] = normalize_object(k8s_cluster, self.remote_objects[ref], ignore_for_diffs)

        logger.info("Diffing remote/old objects against applied/new objects")
        new_objects = []
        changed_objects = []
        with MyThreadPoolExecutor() as executor:
            futures = {}
            for ref, o in diff_objects.items():
                normalized_remote_object = normalized_remote_objects.get(ref)
                if normalized_remote_object is None:
                    new_objects.append(o)
                    continue
                normalized_diff_object = normalized_diff_objects[ref]
                futures[ref] = o, executor.submit(do_diff_object,
                                                  normalized_remote_object, normalized_diff_object, ignore_order)

            for ref, (o, f) in futures.items():
                changes = f.result()
                if not changes:
                    continue
                changed_objects.append({
                    "new_object": o,
                    "old_object": self.remote_objects[ref],
                    "changes": changes
                })

        return new_objects, changed_objects

    def do_replace_object(self, k8s_cluster, ref, callback):
        first_call = True
        while True:
            if first_call:
                o = self.remote_objects.get(ref)
            else:
                o, warnings = k8s_cluster.get_single_object(ref)
            if o is None:
                return None
            first_call = False

            o2 = callback(o)
            if o == o2:
                return o

            try:
                result, warnings = k8s_cluster.replace_object(o2)
                return result
            except ConflictError:
                logger.info("Conflict while patching %s. Retrying..." % get_long_object_name(o2))
                continue

    def do_finish_futures(self, futures):
        ret = {}
        for ref, f in futures:
            e = f.exception()
            if e is not None:
                self.add_error(ref, str(e))
                continue

            o = f.result()
            if o is None:
                continue
            ref = get_object_ref(o)
            ret[ref] = o
        return ret

    def find_rendered_images(self):
        ret = {}
        for d in self.deployments:
            for o in d.objects:
                containers = o.get('spec', {}).get('template', {}).get('spec', {}).get('containers', [])
                for c in containers:
                    image = c.get("image")
                    if not image:
                        continue
                    ret.setdefault(get_object_ref(o), []).append(image)
        return ret

    def add_warnings(self, ref, warnings):
        for w in warnings:
            item = DeployErrorItem(ref=ref, message=w)
            if item not in self.warnings:
                logger.warning("%s: Warning while performing api call. message=%s" % (get_long_object_name_from_ref(ref), w))
                self.warnings.add(item)

    def add_error(self, ref, error):
        ref_str = ""
        if ref is not None:
            ref_str = "%s: " % get_long_object_name_from_ref(ref)
        item = DeployErrorItem(ref=ref, message=error)
        if item not in self.errors:
            logger.error("%sError while performing api call. message=%s" % (ref_str, error))
            self.errors.add(item)

    def clear_errors_and_warnings(self):
        self.errors = set()
        self.warnings = set()

def do_diff_object(old_object, new_object, ignore_order):
    if old_object == new_object:
        return []
    return deep_diff_object(old_object, new_object, ignore_order)
