import dataclasses
import logging

from typing import List, Set, Optional

from kluctl.utils.dict_utils import get_dict_value
from kluctl.utils.k8s_object_utils import get_object_ref, get_long_object_name
from kluctl.utils.utils import parse_bool

logger = logging.getLogger(__name__)

@dataclasses.dataclass
class Hook:
    object: dict
    hooks: Set[str]
    weight: int
    delete_policies: Set[str]
    wait: bool

class HooksUtil:
    def __init__(self, apply_util):
        self.apply_util = apply_util

    def run_hooks(self, d, hooks):
        def do_log(level, str):
            self.apply_util.do_log(d, level, str)

        l = self.get_sorted_hooks_list(d.objects)
        l = [x for x in l if any(h in hooks for h in x.hooks)]

        delete_before_objects = []
        apply_objects = []

        for h in l:
            if self.apply_util.abort_signal:
                return
            if "before-hook-creation" in h.delete_policies:
                delete_before_objects.append(h)
            apply_objects.append(h)

        if len(delete_before_objects) != 0:
            do_log(logging.INFO, "Deleting %d hooks before hook execution" % len(delete_before_objects))
        for h in delete_before_objects:
            do_log(logging.DEBUG, "Deleting hook %s due to hook-delete-policy %s" % (get_long_object_name(h.object), ",".join(h.delete_policies)))
            self.apply_util.delete_object(get_object_ref(h.object))

        if len(apply_objects) != 0:
            do_log(logging.INFO, "Applying %d hooks" % len(delete_before_objects))
        for h in apply_objects:
            if self.apply_util.dry_run and "before-hook-creation" in h.delete_policies:
                self.apply_util.handle_result(h.object, [])
                continue
            do_log(logging.DEBUG, "Applying hook %s" % get_long_object_name(h.object))
            self.apply_util.apply_object(h.object)

        wait_results = {}
        for h in l:
            if self.apply_util.abort_signal:
                return
            ref = get_object_ref(h.object)
            if self.apply_util.had_error(ref):
                continue
            if not h.wait:
                continue
            wait_results[ref] = self.apply_util.wait_object(ref)

        delete_after_objects = []
        for h in reversed(l):
            ref = get_object_ref(h.object)
            if ref not in wait_results:
                continue
            do_delete = False
            if wait_results[ref] and "hook-succeeded" in h.delete_policies:
                do_delete = True
            elif not wait_results[ref] and "hook-failed" in h.delete_policies:
                do_delete = True
            if do_delete:
                delete_after_objects.append(h)

        if len(delete_after_objects) != 0:
            do_log(logging.INFO, "Deleting %d hooks after hook execution" % len(delete_after_objects))
        for h in delete_after_objects:
            do_log(logging.DEBUG, "Deleting hook %s due to hook-delete-policy %s" % (get_long_object_name(h.object), ",".join(h.delete_policies)))
            self.apply_util.delete_object(get_object_ref(h.object))

    def get_hook(self, o) -> Optional[Hook]:
        def get_list(path):
            s = get_dict_value(o, path, "")
            s = s.split(",")
            s = [x for x in s if x != ""]
            return s

        supported_kluctl_hooks = {"pre-deploy", "post-deploy",
                                  "pre-deploy-initial", "post-deploy-initial",
                                  "pre-deploy-upgrade", "post-deploy-upgrade"}
        supported_kluctl_delete_policies = {"before-hook-creation",
                                            "hook-succeeded",
                                            "hook-failed"}

        # delete/rollback hooks are actually not supported, but we won't show warnings about that to not spam the user
        supported_helm_hooks = {"pre-install", "post-install",
                                "pre-upgrade", "post-upgrade",
                                "pre-delete", "post-delete",
                                "pre-rollback", "post-rollback"}

        hooks = get_list('metadata.annotations."kluctl.io/hook"')
        for h in hooks:
            if h not in supported_kluctl_hooks:
                self.apply_util.handle_error(get_object_ref(o), "Unsupported kluctl.io/hook '%s'" % h)

        helm_hooks = get_list('metadata.annotations."helm.sh/hook"')
        for h in helm_hooks:
            if h not in supported_helm_hooks:
                self.apply_util.deployment_collection.add_api_warnings(get_object_ref(o), ["Unsupported helm.sh/hook '%s'" % h])

        if "pre-install" in helm_hooks:
            hooks.append("pre-deploy-initial")
        if "pre-upgrade" in helm_hooks:
            hooks.append("pre-deploy-upgrade")
        if "post-install" in helm_hooks:
            hooks.append("post-deploy-initial")
        if "post-upgrade" in helm_hooks:
            hooks.append("post-deploy-upgrade")
        if "pre-delete" in helm_hooks:
            hooks.append("pre-delete")
        if "post-delete" in helm_hooks:
            hooks.append("post-delete")
        hooks = set(hooks)

        weight = get_dict_value(o, 'metadata.annotations."kluctl.io/hook-weight"')
        if weight is None:
            weight = get_dict_value(o, 'metadata.annotations."helm.sh/hook-weight"')
        if weight is None:
            weight = "0"
        weight = int(weight)

        delete_policy = get_list('metadata.annotations."kluctl.io/hook-delete-policy"')
        delete_policy += get_list('metadata.annotations."helm.sh/hook-delete-policy"')
        if not delete_policy:
            delete_policy = ["before-hook-creation"]
        delete_policy = set(delete_policy)

        for p in delete_policy:
            if p not in supported_kluctl_delete_policies:
                self.apply_util.handle_error(get_object_ref(o), "Unsupported kluctl.io/hook-delete-policy '%s'" % p)

        try:
            wait = parse_bool(get_dict_value(o, 'metadata.annotations."kluctl.io/hook-wait"', "true"), do_raise=True)
        except ValueError as e:
            self.apply_util.handle_error(get_object_ref(o), str(e))
            wait = True

        if not hooks:
            return None

        return Hook(object=o, hooks=hooks, weight=weight, delete_policies=delete_policy, wait=wait)

    def get_sorted_hooks_list(self, l) -> List[Hook]:
        ret = []
        for x in l:
            h = self.get_hook(x)
            if h is None:
                continue
            ret.append(h)
        ret.sort(key=lambda x: x.weight)
        return ret
