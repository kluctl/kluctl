package deployment

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sort"
	"strconv"
	"strings"
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

type hooksUtil struct {
	a *applyUtil
}

type hook struct {
	object         *unstructured.Unstructured
	hooks          map[string]bool
	weight         int
	deletePolicies map[string]bool
	wait           bool
}

func (u *hooksUtil) runHooks(d *deploymentItem, hooks []string) {
	doLog := func(level log.Level, s string, f ...interface{}) {
		u.a.doLog(d, level, s, f...)
	}

	var l []*hook
	for _, h := range u.getSortedHooksList(d.objects) {
		for h2 := range h.hooks {
			if utils.FindStrInSlice(hooks, h2) != -1 {
				l = append(l, h)
				break
			}
		}
	}

	if len(l) != 0 && log.IsLevelEnabled(log.DebugLevel) {
		doLog(log.DebugLevel, "Sorted hooks:")
		for _, h := range l {
			doLog(log.DebugLevel, "  %s", types.RefFromObject(h.object).String())
		}
	}

	var deleteBeforeObjects []*hook
	var applyObjects []*hook

	for _, h := range l {
		if u.a.abortSignal {
			return
		}
		if _, ok := h.deletePolicies["before-hook-creation"]; ok {
			deleteBeforeObjects = append(deleteBeforeObjects, h)
		}
		applyObjects = append(applyObjects, h)
	}

	doDeleteForPolicy := func(h *hook) bool {
		ref := types.RefFromObject(h.object)
		var dpStr []string
		for p := range h.deletePolicies {
			dpStr = append(dpStr, p)
		}
		doLog(log.DebugLevel, "Deleting hook %s due to hook-delete-policy %s", ref.String(), strings.Join(dpStr, ","))
		return u.a.deleteObject(ref)
	}

	if len(deleteBeforeObjects) != 0 {
		doLog(log.InfoLevel, "Deleting %d hooks before hook execution", len(deleteBeforeObjects))
	}
	for _, h := range deleteBeforeObjects {
		if !doDeleteForPolicy(h) {
			return
		}
	}

	waitResults := make(map[types.ObjectRef]bool)
	if len(applyObjects) != 0 {
		doLog(log.InfoLevel, "Applying %d hooks", len(applyObjects))
	}
	for _, h := range applyObjects {
		ref := types.RefFromObject(h.object)
		_, replaced := h.deletePolicies["before-hook-creation"]
		doLog(log.DebugLevel, "Applying hook %s", ref.String())
		u.a.applyObject(h.object, replaced, true)

		if u.a.hadError(ref) {
			continue
		}
		if !h.wait {
			continue
		}
		waitResults[ref] = u.a.waitHook(ref)
	}

	var deleteAfterObjects []*hook
	for i := len(applyObjects) - 1; i >= 0; i-- {
		h := applyObjects[i]
		ref := types.RefFromObject(h.object)
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
		doLog(log.InfoLevel, "Deleting %d hooks after hook execution", len(deleteAfterObjects))
	}
	for _, h := range deleteAfterObjects {
		doDeleteForPolicy(h)
	}
}

func (u *hooksUtil) getHook(o *unstructured.Unstructured) *hook {
	ref := types.RefFromObject(o)
	getSet := func(name string) map[string]bool {
		ret := make(map[string]bool)
		a, ok := o.GetAnnotations()[name]
		if !ok {
			return ret
		}
		for _, x := range strings.Split(a, ",") {
			if x != "" {
				ret[x] = true
			}
		}
		return ret
	}

	hooks := getSet("kluctl.io/hook")
	for h := range hooks {
		if utils.FindStrInSlice(supportedKluctlHooks, h) == -1 {
			u.a.handleError(ref, fmt.Errorf("unsupported kluctl.io/hook '%s'", h))
		}
	}

	helmHooks := getSet("helm.sh/hook")
	for h := range helmHooks {
		if utils.FindStrInSlice(supportedHelmHooks, h) == -1 {
			u.a.handleError(ref, fmt.Errorf("unsupported helm.sh/hook '%s'", h))
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

	weightStr, ok := o.GetAnnotations()["kluctl.io/hook-weight"]
	if !ok {
		weightStr, ok = o.GetAnnotations()["helm.sh/hook-weight"]
	}
	if !ok {
		weightStr = "0"
	}
	weight, err := strconv.ParseInt(weightStr, 10, 32)
	if err != nil {
		u.a.handleError(ref, fmt.Errorf("failed to parse hook weight: %w", err))
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
			u.a.handleError(ref, fmt.Errorf("unsupported kluctl.io/hook-delete-policy '%s'", p))
		}
	}

	waitStr, ok := o.GetAnnotations()["kluctl.io/hook-wait"]
	if !ok {
		waitStr = "true"
	}
	wait, err := strconv.ParseBool(waitStr)
	if err != nil {
		u.a.handleError(ref, fmt.Errorf("failed to parse %s as bool", waitStr))
		wait = true
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
	}
}

func (u *hooksUtil) getSortedHooksList(objects []*unstructured.Unstructured) []*hook {
	var ret []*hook
	for _, o := range objects {
		h := u.getHook(o)
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
