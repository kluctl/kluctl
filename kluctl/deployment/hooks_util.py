from kluctl.utils.dict_utils import get_dict_value
from kluctl.utils.k8s_object_utils import get_object_ref

class HooksUtil:
    def __init__(self, apply_util):
        self.apply_util = apply_util

    def run_hooks(self, d, hook):
        l = self.get_sorted_hooks_list(d.objects)
        l = [x for x in l if hook in x[1]]

        for x, hooks, weight, delete_policy in l:
            if self.apply_util.abort_signal:
                return
            if "before-hook-creation" in delete_policy:
                self.apply_util.delete_object(get_object_ref(x))

        for x, hooks, weight, delete_policy in l:
            if self.apply_util.abort_signal:
                return
            self.apply_util.apply_object(x)

        wait_results = {}
        for x, hooks, weight, delete_policy in l:
            if self.apply_util.abort_signal:
                return
            ref = get_object_ref(x)
            if self.apply_util.had_error(ref):
                continue
            wait_results[ref] = self.apply_util.wait_object(ref)

        for x, hooks, weight, delete_policy in reversed(l):
            ref = get_object_ref(x)
            if ref not in wait_results:
                continue
            if wait_results[ref]:
                if "hook-succeeded" in delete_policy:
                    self.apply_util.delete_object(ref)
            else:
                if "hook-failed" in delete_policy:
                    self.apply_util.delete_object(ref)

    def get_hooks(self, o):
        def get_list(path):
            s = get_dict_value(o, path, "")
            s = s.split(",")
            s = [x for x in s if x != ""]
            return s

        supported_kluctl_hooks = {"pre-deploy", "post-deploy",
                                  "pre-deploy-initial", "post-deploy-initial"}
        supported_kluctl_delete_policies = {"before-hook-creation",
                                            "hook-succeeded",
                                            "hook-failed"}

        # delete/rollback hooks are actually not supported, but we won't show warnings about that to not spam the user
        supported_helm_hooks = {"pre-install", "post-install",
                                "pre-upgrade", "post-upgrade",
                                "pre-delete", "post-delete",
                                "pre-rollback", "post-rollback"}

        hooks = get_list("metadata.annotations.kluctl\\.io/hook")
        for h in hooks:
            if h not in supported_kluctl_hooks:
                self.apply_util.handle_error(get_object_ref(o), "Unsupported kluctl.io/hook '%s'" % h)

        helm_hooks = get_list("metadata.annotations.helm\\.sh/hook")
        for h in helm_hooks:
            if h not in supported_helm_hooks:
                self.apply_util.deployment_collection.add_api_warnings("Unsupported helm.sh/hook '%s'" % h)

        if "pre-install" in helm_hooks:
            hooks.append("pre-deploy-initial")
        if "pre-upgrade" in helm_hooks:
            hooks.append("pre-deploy")
        if "post-install" in helm_hooks:
            hooks.append("post-deploy-initial")
        if "post-upgrade" in helm_hooks:
            hooks.append("post-deploy")
        if "pre-delete" in helm_hooks:
            hooks.append("pre-delete")
        if "post-delete" in helm_hooks:
            hooks.append("post-delete")
        hooks = set(hooks)

        weight = get_dict_value(o, "metadata.annotations.kluctl\\.io/hook-weight")
        if weight is None:
            weight = get_dict_value(o, "metadata.annotations.helm\\.sh/hook-weight")
        if weight is None:
            weight = "0"
        weight = int(weight)

        delete_policy = get_list("metadata.annotations.kluctl\\.io/hook-delete-policy")
        delete_policy += get_list("metadata.annotations.helm\\.sh/hook-delete-policy")
        if not delete_policy:
            delete_policy = ["before-hook-creation"]
        delete_policy = set(delete_policy)

        for p in delete_policy:
            if p not in supported_kluctl_delete_policies:
                self.apply_util.handle_error(get_object_ref(o), "Unsupported kluctl.io/hook-delete-policy '%s'" % h)

        return hooks, weight, delete_policy

    def get_sorted_hooks_list(self, l):
        ret = []
        for x in l:
            hooks, weight, delete_policy = self.get_hooks(x)
            ret.append((x, hooks, weight, delete_policy))
        ret.sort(key=lambda x: x[2])
        return ret
