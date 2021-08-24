import dataclasses
import itertools
import logging
from concurrent.futures import Future
from typing import Optional

from kubernetes.client import ApiException
from kubernetes.dynamic.exceptions import ResourceNotFoundError, ConflictError

from kluctl.deployment.kustomize_deployment import KustomizeDeployment
from kluctl.diff.k8s_diff import deep_diff_object
from kluctl.diff.managed_fields import remove_non_managed_fields2
from kluctl.diff.normalize import normalize_object
from kluctl.deployment.images import SeenImages
from kluctl.utils.dict_utils import copy_dict, get_dict_value
from kluctl.utils.k8s_delete_utils import find_objects_for_delete
from kluctl.utils.k8s_object_utils import get_object_ref, get_long_object_name, get_long_object_name_from_ref, \
    ObjectRef, remove_namespace_from_ref_if_needed
from kluctl.utils.status_validation import validate_object, ValidateResult, ValidateResultItem
from kluctl.utils.templated_dir import TemplatedDir
from kluctl.utils.utils import MyThreadPoolExecutor

logger = logging.getLogger(__name__)

@dataclasses.dataclass(frozen=True, eq=True)
class DeployErrorItem:
    ref: ObjectRef
    reason: str
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
        self.seen_images = SeenImages(images)
        self.inclusion = inclusion
        self.tmpdir = tmpdir
        self.for_seal = for_seal
        self.deployments = self._collect_deployments(self.project)

        self.is_rendered = False
        self.is_built = False
        self.remote_objects = {}

        self.api_errors = set()
        self.api_warnings = set()

    def _collect_deployments(self, project):
        ret = []

        for c in project.conf['kustomizeDirs']:
            deployment = KustomizeDeployment(project, self, c)
            ret.append(deployment)

        for inc in project.conf['includes']:
            if get_dict_value(inc, "barrier", False):
                deployment = KustomizeDeployment(project, self, {"barrier": True})
                ret.append(deployment)

            d = inc.get('_included_deployment_collection')
            if d is not None:
                ret += self._collect_deployments(d)

        return ret

    def render_deployments(self):
        if self.is_rendered:
            return
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

            if not self.for_seal:
                for d in self.deployments:
                    d.resolve_sealed_secrets()
        self.is_rendered = True

    def build_kustomize_objects(self):
        if self.is_built:
            return
        logger.info("Building kustomize objects")

        with MyThreadPoolExecutor() as executor:
            jobs = []
            for d in self.deployments:
                jobs.append(executor.submit(d.build_objects))
            for job in jobs:
                job.result()
        self.is_built = True

    def update_remote_objects(self, k8s_cluster):
        logger.info("Updating remote objects")
        refs = set()
        for ref in self.local_objects_by_ref().keys():
            if ref not in self.remote_objects:
                refs.add(ref)
        r = k8s_cluster.get_objects_by_object_refs(refs)
        for o, w in r:
            self.remote_objects[get_object_ref(o)] = o
            self.add_api_warnings(get_object_ref(o), w)

    def forget_remote_objects(self, refs):
        for ref in refs:
            self.remote_objects.pop(ref, None)

    def local_objects_by_ref(self):
        flat = list(itertools.chain(*[d.objects for d in self.deployments]))
        by_ref = dict((get_object_ref(x), x) for x in flat)
        return by_ref

    def deploy(self, k8s_cluster, force_apply, replace_on_error, abort_on_error):
        self.clear_errors_and_warnings()
        self.render_deployments()
        self.build_kustomize_objects()
        applied_objects = self.do_deploy(k8s_cluster, force_apply, replace_on_error,
                                         False, abort_on_error)
        new_objects, changed_objects = self.do_diff(k8s_cluster, applied_objects, False, False, False, False)
        return DeployDiffResult(new_objects=new_objects, changed_objects=changed_objects,
                                errors=list(self.api_errors), warnings=list(self.api_warnings))

    def diff(self, k8s_cluster, force_apply, replace_on_error, ignore_tags, ignore_labels, ignore_annotations, ignore_order):
        self.clear_errors_and_warnings()
        self.render_deployments()
        self.build_kustomize_objects()
        applied_objects = self.do_deploy(k8s_cluster, force_apply, replace_on_error,
                                         True, False)
        new_objects, changed_objects = self.do_diff(k8s_cluster, applied_objects, ignore_tags, ignore_labels, ignore_annotations, ignore_order)
        return DeployDiffResult(new_objects=new_objects, changed_objects=changed_objects,
                                errors=list(self.api_errors), warnings=list(self.api_warnings))

    def validate(self, k8s_cluster):
        self.clear_errors_and_warnings()
        self.render_deployments()
        self.build_kustomize_objects()
        self.update_remote_objects(k8s_cluster)
        result = ValidateResult()

        for w in self.api_warnings:
            result.warnings.append(ValidateResultItem(ref=w.ref, reason=w.reason, message=w.message))
        for e in self.api_errors:
            result.errors.append(ValidateResultItem(ref=e.ref, reason=e.reason, message=e.message))

        for d in self.deployments:
            if not d.check_inclusion_for_deploy():
                continue
            for o in d.objects:
                ref = get_object_ref(o)
                remote_object = self.remote_objects.get(ref)
                if not remote_object:
                    result.errors.append(ValidateResultItem(ref=ref, reason="not-found", message="Object not found"))
                    continue
                r = validate_object(remote_object)
                result.errors += r.errors
                result.warnings += r.warnings
                result.results += r.results
        return result

    def find_delete_objects(self, k8s_cluster):
        self.clear_errors_and_warnings()
        labels = self.project.get_delete_by_labels()
        return find_objects_for_delete(k8s_cluster, labels, self.inclusion, [])

    def find_purge_objects(self, k8s_cluster):
        self.clear_errors_and_warnings()
        self.render_deployments()
        self.build_kustomize_objects()
        logger.info("Searching objects not found in local objects")
        labels = self.project.get_delete_by_labels()
        excluded_objects = list(self.local_objects_by_ref().keys())
        return find_objects_for_delete(k8s_cluster, labels, self.inclusion, excluded_objects)

    def do_patch(self, k8s_cluster, force_apply, replace_on_error, dry_run, abort_on_error):
        logger.info("Running server-side apply for all objects%s", k8s_cluster.get_dry_run_suffix(dry_run))

        jobs = []

        @dataclasses.dataclass(init=False)
        class Job:
            object: dict
            future: Future
            result_object: Optional[dict]
            result_exception: Optional[Exception]
            done: bool = False

        def add_job(o, f):
            job = Job()
            job.object = o
            job.future = f
            jobs.append(job)

        def finish_jobs():
            for job in jobs:
                if job.done:
                    continue
                job.result_exception = job.future.exception()
                if job.result_exception is None:
                    job.result = job.future.result()
                job.done = True

        with MyThreadPoolExecutor(max_workers=16) as executor:
            previous_was_barrier = False
            for d in self.deployments:
                if self.api_errors and abort_on_error:
                    break

                if previous_was_barrier:
                    logger.info("Waiting on barrier...")
                    finish_jobs()
                previous_was_barrier = get_dict_value(d.config, "barrier", False)

                include = d.check_inclusion_for_deploy()
                if "path" not in d.config:
                    continue
                if not include:
                    logger.info("Skipping kustomizeDir %s" % d.get_rel_kustomize_dir())
                    continue
                logger.info("Applying kustomizeDir '%s' with %d objects" % (d.get_rel_kustomize_dir(), len(d.objects)))

                for x in d.objects:
                    if self.api_errors and abort_on_error:
                        break
                    logger.debug(f"  {get_long_object_name(x)}")

                    # replaced means it is deleted and then re-created
                    is_replaced = self.is_replace_needed(x)

                    if not force_apply and not is_replaced:
                        x2 = remove_non_managed_fields2(x, self.remote_objects)
                    else:
                        x2 = x

                    if dry_run and is_replaced:
                        # The necessary delete was not really performed in case we are in dry_run mode
                        def dummy(x3):
                            # let's pretend that we applied it
                            return x3, []
                        f = executor.submit(dummy, x2)
                    else:
                        f = executor.submit(k8s_cluster.patch_object, x2, force_dry_run=dry_run, force_apply=True)
                    add_job(x2, f)
            finish_jobs()

        applied_objects = {}
        for job in jobs:
            if job.result_exception is not None:
                try:
                    raise job.result_exception
                except ResourceNotFoundError as e:
                    ref = get_object_ref(job.object)
                    self.add_api_error(ref, k8s_cluster.get_status_message(e))
                except ApiException as e:
                    ref = get_object_ref(job.object)
                    if not replace_on_error:
                        self.add_api_error(ref, k8s_cluster.get_status_message(e))
                        continue
                    logger.info("Patching %s failed, retrying by deleting and re-applying" %
                                get_long_object_name_from_ref(ref))
                    try:
                        k8s_cluster.delete_single_object(ref, force_dry_run=dry_run, ignore_not_found=True)
                        if not dry_run and not k8s_cluster.dry_run:
                            r, patch_warnings = k8s_cluster.patch_object(job.object, force_apply=True)
                            # We must use the ref from the applied object as k8s might remove the namespace field
                            ref = get_object_ref(r)
                            applied_objects[ref] = r
                            self.add_api_warnings(ref, patch_warnings)
                        else:
                            applied_objects[ref] = job.object
                    except ApiException as e2:
                        self.add_api_error(ref, k8s_cluster.get_status_message(e2))
                continue

            r, patch_warnings = job.result
            # We must use the ref from the applied object as k8s might remove the namespace field
            ref = get_object_ref(r)
            applied_objects[ref] = r
            self.add_api_warnings(ref, patch_warnings)

        return applied_objects

    def do_deploy(self, k8s_cluster, force_apply, replace_on_error, dry_run, abort_on_error):
        self.update_remote_objects(k8s_cluster)

        replaced_objects = self.find_replaced_objects()
        if replaced_objects:
            futures = []
            with MyThreadPoolExecutor(max_workers=16) as executor:
                logger.info("Deleting %d objects which need replacement (delete+create)" % len(replaced_objects))
                for o in replaced_objects:
                    ref = get_object_ref(o)
                    f = executor.submit(k8s_cluster.delete_single_object, ref, force_dry_run=dry_run, ignore_not_found=True)
                    futures.append((ref, f))
                for ref, f in futures:
                    try:
                        f.result()
                    except Exception as e:
                        self.add_api_error(ref, str(e))
                        logger.error("Failed to delete %s. %s" % (get_long_object_name_from_ref(ref), e))
        if abort_on_error and self.api_errors:
            return {}

        # TODO remove this
        self.migrate_to_new_manager(k8s_cluster)

        return self.do_patch(k8s_cluster, force_apply, replace_on_error, dry_run, abort_on_error)

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
                ref = remove_namespace_from_ref_if_needed(k8s_cluster, ref)
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

    def is_replace_needed(self, o):
        hook = get_dict_value(o, "metadata.annotations.helm\\.sh/hook")
        if not hook:
            return False
        if all(h in ["pre-install", "post-install", "pre-delete", "post-delete", "pre-upgrade", "post-upgrade", "pre-rollback", "post-rollback", "test"] for h in hook.split(",")):
            return True
        return False

    def find_replaced_objects(self):
        self.render_deployments()
        self.build_kustomize_objects()
        ret = []
        for d in self.deployments:
            for o in d.objects:
                ref = get_object_ref(o)
                if ref not in self.remote_objects:
                    continue
                if not d.check_inclusion_for_deploy():
                    continue
                if self.is_replace_needed(o):
                    ret.append(o)
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

    def add_api_warnings(self, ref, warnings):
        for w in warnings:
            logger.warning("%s: Warning while performing api call. message=%s" % (get_long_object_name_from_ref(ref), w))
            self.api_warnings.add(DeployErrorItem(ref=ref, reason="api", message=w))

    def add_api_error(self, ref, error):
        logger.error("%s: Error while performing api call. message=%s" % (get_long_object_name_from_ref(ref), error))
        self.api_errors.add(DeployErrorItem(ref=ref, reason="api", message=error))

    def clear_errors_and_warnings(self):
        self.api_errors = set()
        self.api_warnings = set()

    # TODO remove this when legacy deployments are all migrated
    def migrate_to_new_manager(self, k8s_cluster):

        def do_replace(o):
            while True:
                o2 = copy_dict(o)
                need_replace = False
                for mf in get_dict_value(o2, "metadata.managedFields", []):
                    if mf["manager"] == "deployctl":
                        mf["manager"] = "kluctl"
                        need_replace = True
                        break
                if not need_replace:
                    break

                try:
                    k8s_cluster.replace_object(o2)
                    logger.info("Migrated %s to new kluctl field manager" % get_long_object_name(o2))
                    break
                except ConflictError:
                    o, _ = k8s_cluster.get_single_object(get_object_ref(o))
                    logger.info("Conflict while migrating %s to new kluctl field manager" % get_long_object_name(o2))
                    continue

        with MyThreadPoolExecutor(max_workers=8) as executor:
            for d in self.deployments:
                if not d.check_inclusion_for_deploy():
                    continue
                for o in d.objects:
                    o2 = self.remote_objects.get(get_object_ref(o))
                    if o2 is None:
                        continue
                    executor.submit(do_replace, o2)

def do_diff_object(old_object, new_object, ignore_order):
    if old_object == new_object:
        return []
    return deep_diff_object(old_object, new_object, ignore_order)
