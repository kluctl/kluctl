import logging
import threading

from kubernetes.client import ApiException
from kubernetes.dynamic.exceptions import ResourceNotFoundError

from kluctl.deployment.hooks_util import HooksUtil
from kluctl.diff.managed_fields import resolve_field_manager_conflicts
from kluctl.utils.dict_utils import get_dict_value, set_dict_value
from kluctl.utils.k8s_object_utils import get_object_ref, get_long_object_name, get_long_object_name_from_ref
from kluctl.utils.k8s_status_validation import validate_object
from kluctl.utils.utils import MyThreadPoolExecutor

logger = logging.getLogger(__name__)

class ApplyUtil:
    def __init__(self, deployment_collection, k8s_cluster, force_apply, replace_on_error, force_replace_on_error, dry_run, abort_on_error):
        self.deployment_collection = deployment_collection
        self.k8s_cluster = k8s_cluster
        self.force_apply = force_apply
        self.replace_on_error = replace_on_error or force_replace_on_error
        self.force_replace_on_error = force_replace_on_error
        self.dry_run = dry_run
        self.abort_on_error = abort_on_error

        self.applied_objects = {}
        self.abort_signal = False
        self.error_refs = {}
        self.mutex = threading.Lock()

    def handle_result(self, applied_object, patch_warnings):
        with self.mutex:
            ref = get_object_ref(applied_object)
            self.applied_objects[ref] = applied_object
            self.deployment_collection.add_api_warnings(ref, patch_warnings)

    def handle_error(self, ref, error):
        with self.mutex:
            self.error_refs[ref] = error
            if self.abort_on_error:
                self.abort_signal = True
            self.deployment_collection.add_api_error(ref, error)

    def had_error(self, ref):
        with self.mutex:
            return ref in self.error_refs

    def delete_object(self, ref):
        self.k8s_cluster.delete_single_object(ref, force_dry_run=self.dry_run, ignore_not_found=True)

    def apply_object(self, x, replaced):
        logger.debug(f"  {get_long_object_name(x)}")

        x2 = self.k8s_cluster.fix_object_for_patch(x)
        if not self.force_apply:
            ref = get_object_ref(x)
            remote_object = self.deployment_collection.remote_objects.get(ref)
            x2, overwritten = resolve_field_manager_conflicts(x2, remote_object)
            warnings = []
            for ow in overwritten:
                warnings.append("Field '%s' is now owned by field manager '%s'. "
                                "It is NOT updated to the desired value '%s'!" % (ow.path, ow.field_manager, ow.local_value))
            self.deployment_collection.add_api_warnings(ref, warnings)

        if self.dry_run and replaced and get_object_ref(x) in self.deployment_collection.remote_objects:
            # Let's simulate that this object was deleted in dry-run mode. If we'd actually try a dry-run apply with
            # this object, it might fail as it is expected to not exist.
            self.handle_result(x2, [])
            return

        try:
            r, patch_warnings = self.k8s_cluster.patch_object(x2, force_dry_run=self.dry_run, force_apply=True)
            self.handle_result(r, patch_warnings)
        except ResourceNotFoundError as e:
            ref = get_object_ref(x)
            self.handle_error(ref, self.k8s_cluster.get_status_message(e))
        except ApiException as e:
            ref = get_object_ref(x)
            if not self.replace_on_error:
                self.handle_error(ref, self.k8s_cluster.get_status_message(e))
                return
            logger.info("Patching %s failed, retrying with replace instead of patch" %
                        get_long_object_name_from_ref(ref))
            try:
                remote_object = self.deployment_collection.remote_objects.get(ref)
                if remote_object is None:
                    self.handle_error(ref, self.k8s_cluster.get_status_message(e))
                    return
                resource_version = get_dict_value(remote_object, "metadata.resourceVersion")
                x2 = set_dict_value(x, "metadata.resourceVersion", resource_version, do_clone=True)
                r, patch_warnings = self.k8s_cluster.replace_object(x2, force_dry_run=self.dry_run)
                self.handle_result(r, patch_warnings)
            except ApiException as e2:
                self.handle_error(ref, self.k8s_cluster.get_status_message(e2))

                if not self.force_replace_on_error:
                    self.handle_error(ref, self.k8s_cluster.get_status_message(e))
                    return

                logger.info("Patching %s failed, retrying by deleting and re-applying" %
                            get_long_object_name_from_ref(ref))
                try:
                    self.k8s_cluster.delete_single_object(ref, force_dry_run=self.dry_run, ignore_not_found=True)
                    if not self.dry_run:
                        r, patch_warnings = self.k8s_cluster.patch_object(x, force_apply=True)
                        self.handle_result(r, patch_warnings)
                    else:
                        self.handle_result(x, [])
                except ApiException as e2:
                    self.handle_error(ref, self.k8s_cluster.get_status_message(e2))

    def wait_object(self, ref):
        if self.dry_run:
            return True

        did_log = False
        logger.debug("Starting wait for hook %s" % get_long_object_name_from_ref(ref))
        while True:
            o, _ = self.k8s_cluster.get_single_object(ref)
            if o is None:
                self.handle_error(ref, "Object disappeared while waiting for it to become ready")
                return False
            v = validate_object(o, False)
            if v.ready:
                return True
            if v.errors:
                return False

            if not did_log:
                logger.info("Waiting for for hook %s to get ready..." % get_long_object_name_from_ref(ref))
                did_log = True

    def apply_kustomize_deployment(self, d):
        if "path" not in d.config:
            return
        include = d.check_inclusion_for_deploy()
        if not include:
            self.do_log(d, logging.INFO, "Skipping")
            return

        inital_deploy = True
        for o in d.objects:
            if get_object_ref(o) in self.deployment_collection.remote_objects:
                inital_deploy = False
                break

        hook_util = HooksUtil(self)

        if inital_deploy:
            hook_util.run_hooks(d, ["pre-deploy-initial", "pre-deploy"])
        else:
            hook_util.run_hooks(d, ["pre-deploy-upgrade", "pre-deploy"])

        apply_objects = []
        for o in d.objects:
            if hook_util.get_hook(o) is not None:
                continue
            apply_objects.append(o)
        self.do_log(d, logging.INFO, "Applying %d objects" % len(d.objects))
        for o in apply_objects:
            self.apply_object(o, False)

        if inital_deploy:
            hook_util.run_hooks(d, ["post-deploy-initial", "post-deploy"])
        else:
            hook_util.run_hooks(d, ["post-deploy-upgrade", "post-deploy"])

    def apply_deployments(self):
        logger.info("Running server-side apply for all objects%s", self.k8s_cluster.get_dry_run_suffix(self.dry_run))

        futures = []

        def finish_futures():
            for f in futures:
                f.result()

        with MyThreadPoolExecutor(max_workers=16) as executor:
            previous_was_barrier = False
            for d in self.deployment_collection.deployments:
                if self.abort_signal:
                    break

                if previous_was_barrier:
                    logger.info("Waiting on barrier...")
                    finish_futures()
                previous_was_barrier = get_dict_value(d.config, "barrier", False)

                f = executor.submit(self.apply_kustomize_deployment, d)
                futures.append(f)

            finish_futures()

    def do_log(self, d, level, str):
        logger.log(level, "%s: %s" % (d.get_rel_kustomize_dir(), str))
