package utils

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"sort"
	"strconv"
	"strings"
	"time"
)

var supportedKluctlHooks = []string{
	"pre-deploy", "post-deploy",
	"pre-deploy-initial", "post-deploy-initial",
	"pre-deploy-upgrade", "post-deploy-upgrade",
}

var supportedKluctlDeletePolicies = []string{
	"before-hook-creation",
	"hook-succeeded",
	"hook-failed",
}

// delete/rollback hooks are actually not supported, but we won't show warnings about that to not spam the user
var supportedHelmHooks = []string{
	"pre-install", "post-install",
	"pre-upgrade", "post-upgrade",
	"pre-delete", "post-delete",
	"pre-rollback", "post-rollback",
}

type HooksUtil struct {
	a *ApplyUtil
}

func NewHooksUtil(a *ApplyUtil) *HooksUtil {
	return &HooksUtil{a: a}
}

type hook struct {
	object         *uo.UnstructuredObject
	hooks          map[string]bool
	weight         int
	deletePolicies map[string]bool
	wait           bool
	timeout        time.Duration
}

func (u *HooksUtil) DetermineHooks(d *deployment.DeploymentItem, hooks []string) []*hook {
	var l []*hook
	for _, h := range u.getSortedHooksList(d.Objects) {
		for h2 := range h.hooks {
			if utils.FindStrInSlice(hooks, h2) != -1 {
				l = append(l, h)
				break
			}
		}
	}
	return l
}

func (u *HooksUtil) RunHooks(hooks []*hook) {
	var deleteBeforeObjects []*hook
	var applyObjects []*hook

	for _, h := range hooks {
		if u.a.abortSignal.Load().(bool) {
			return
		}
		if _, ok := h.deletePolicies["before-hook-creation"]; ok {
			deleteBeforeObjects = append(deleteBeforeObjects, h)
		}
		applyObjects = append(applyObjects, h)
	}

	doDeleteForPolicy := func(h *hook, i int, cnt int) bool {
		ref := h.object.GetK8sRef()
		var dpStr []string
		for p := range h.deletePolicies {
			dpStr = append(dpStr, p)
		}
		u.a.sctx.UpdateAndInfoFallback("Deleting hook %s due to hook-delete-policy %s (%d of %d)", ref.String(), strings.Join(dpStr, ","), i+1, cnt)
		return u.a.DeleteObject(ref, true)
	}

	if len(deleteBeforeObjects) != 0 {
		u.a.sctx.InfoFallback("Deleting %d hooks before hook execution", len(deleteBeforeObjects))
	}
	for i, h := range deleteBeforeObjects {
		doDeleteForPolicy(h, i, len(deleteBeforeObjects))
	}

	waitResults := make(map[k8s.ObjectRef]bool)
	if len(applyObjects) != 0 {
		u.a.sctx.InfoFallback("Applying %d hooks", len(applyObjects))
	}
	for i, h := range applyObjects {
		ref := h.object.GetK8sRef()
		_, replaced := h.deletePolicies["before-hook-creation"]
		u.a.sctx.UpdateAndInfoFallback("Applying hook %s (%d of %d)", ref.String(), i+1, len(applyObjects))
		u.a.ApplyObject(h.object, replaced, true)
		u.a.sctx.Increment()

		if u.a.HadError(ref) {
			continue
		}
		if !h.wait {
			continue
		}
		waitResults[ref] = u.a.WaitReadiness(ref, h.timeout)
	}

	var deleteAfterObjects []*hook
	for i := len(applyObjects) - 1; i >= 0; i-- {
		h := applyObjects[i]
		ref := h.object.GetK8sRef()
		waitResult, ok := waitResults[ref]
		if !ok {
			continue
		}
		doDelete := false
		if _, ok := h.deletePolicies["hook-succeeded"]; waitResult && ok {
			doDelete = true
		} else if _, ok := h.deletePolicies["hook-failed"]; !waitResult && ok {
			doDelete = true
		}
		if doDelete {
			deleteAfterObjects = append(deleteAfterObjects, h)
		}
	}

	if len(deleteAfterObjects) != 0 {
		u.a.sctx.InfoFallback("Deleting %d hooks after hook execution", len(deleteAfterObjects))
	}
	for i, h := range deleteAfterObjects {
		doDeleteForPolicy(h, i, len(deleteAfterObjects))
	}
}

func (u *HooksUtil) GetHook(o *uo.UnstructuredObject) *hook {
	ref := o.GetK8sRef()
	getSet := func(name string) map[string]bool {
		ret := make(map[string]bool)
		a := o.GetK8sAnnotation(name)
		if a == nil {
			return ret
		}
		for _, x := range strings.Split(*a, ",") {
			if x != "" {
				ret[x] = true
			}
		}
		return ret
	}

	hooks := getSet("kluctl.io/hook")
	for h := range hooks {
		if utils.FindStrInSlice(supportedKluctlHooks, h) == -1 {
			u.a.HandleError(ref, fmt.Errorf("unsupported kluctl.io/hook '%s'", h))
		}
	}

	helmHooks := getSet("helm.sh/hook")
	for h := range helmHooks {
		if utils.FindStrInSlice(supportedHelmHooks, h) == -1 {
			u.a.HandleWarning(ref, fmt.Errorf("unsupported helm.sh/hook '%s'", h))
		}
	}

	helmCompatibility := func(helmName string, kluctlName string) {
		if _, ok := helmHooks[helmName]; ok {
			hooks[kluctlName] = true
		}
	}

	helmCompatibility("pre-install", "pre-deploy-initial")
	helmCompatibility("pre-upgrade", "pre-deploy-upgrade")
	helmCompatibility("post-install", "post-deploy-initial")
	helmCompatibility("post-upgrade", "post-deploy-upgrade")
	helmCompatibility("pre-delete", "pre-delete")
	helmCompatibility("post-delete", "post-delete")

	weightStr := o.GetK8sAnnotation("kluctl.io/hook-weight")
	if weightStr == nil {
		weightStr = o.GetK8sAnnotation("helm.sh/hook-weight")
	}
	if weightStr == nil {
		x := "0"
		weightStr = &x
	}
	weight, err := strconv.ParseInt(*weightStr, 10, 32)
	if err != nil {
		u.a.HandleError(ref, fmt.Errorf("failed to parse hook weight: %w", err))
	}

	deletePolicy := getSet("kluctl.io/hook-delete-policy")
	for d := range getSet("helm.sh/hook-delete-policy") {
		deletePolicy[d] = true
	}
	if len(deletePolicy) == 0 {
		deletePolicy["before-hook-creation"] = true
	}

	for p := range deletePolicy {
		if utils.FindStrInSlice(supportedKluctlDeletePolicies, p) == -1 {
			u.a.HandleError(ref, fmt.Errorf("unsupported kluctl.io/hook-delete-policy '%s'", p))
		}
	}

	waitStr := o.GetK8sAnnotation("kluctl.io/hook-wait")
	if waitStr == nil {
		x := "true"
		waitStr = &x
	}
	wait, err := strconv.ParseBool(*waitStr)
	if err != nil {
		u.a.HandleError(ref, fmt.Errorf("failed to parse %s as bool", *waitStr))
		wait = true
	}

	timeoutStr := o.GetK8sAnnotation("kluctl.io/hook-timeout")
	var timeout time.Duration
	if timeoutStr != nil {
		t, err := time.ParseDuration(*timeoutStr)
		if err != nil {
			u.a.HandleError(ref, fmt.Errorf("failed to parse duration: %w", err))
		} else {
			timeout = t
		}
	}

	if len(hooks) == 0 {
		return nil
	}

	return &hook{
		object:         o,
		hooks:          hooks,
		weight:         int(weight),
		deletePolicies: deletePolicy,
		wait:           wait,
		timeout:        timeout,
	}
}

func (u *HooksUtil) getSortedHooksList(objects []*uo.UnstructuredObject) []*hook {
	var ret []*hook
	for _, o := range objects {
		h := u.GetHook(o)
		if h == nil {
			continue
		}
		ret = append(ret, h)
	}
	sort.SliceStable(ret, func(i, j int) bool {
		return ret[i].weight < ret[j].weight
	})
	return ret
}

func (h *hook) IsPersistent() bool {
	for p := range h.deletePolicies {
		if p != "before-hook-creation" && p != "hook-failed" {
			return false
		}
	}
	return true
}
