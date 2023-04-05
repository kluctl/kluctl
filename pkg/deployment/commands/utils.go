package commands

import (
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/deployment/utils"
	"github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"sort"
)

func collectObjects(c *deployment.DeploymentCollection, ru *utils.RemoteObjectUtils, au *utils.ApplyDeploymentsUtil, du *utils.DiffUtil, orphans []k8s.ObjectRef, deleted []k8s.ObjectRef) []result.ResultObject {
	m := map[k8s.ObjectRef]*result.ResultObject{}

	getOrCreate := func(ref k8s.ObjectRef) *result.ResultObject {
		x, ok := m[ref]
		if !ok {
			x = &result.ResultObject{}
			x.Ref = ref
			m[ref] = x
		}
		return x
	}

	if c != nil {
		for _, x := range c.LocalObjects() {
			o := getOrCreate(x.GetK8sRef())
			o.Rendered = x
		}
	}
	if ru != nil {
		for _, x := range ru.GetFilteredRemoteObjects(nil) {
			o := getOrCreate(x.GetK8sRef())
			o.Remote = x
		}
	}

	if au != nil {
		for _, x := range au.GetAppliedObjects() {
			o := getOrCreate(x.GetK8sRef())
			o.Applied = x
		}

		for _, x := range au.GetAppliedHookObjects() {
			o := getOrCreate(x.GetK8sRef())
			o.Hook = true
		}
		for _, x := range au.GetNewObjectRefs() {
			o := getOrCreate(x)
			o.New = true
		}
		for _, x := range au.GetDeletedObjects() {
			o := getOrCreate(x)
			o.Deleted = true
		}
	}
	if du != nil {
		for _, x := range du.ChangedObjects {
			o := getOrCreate(x.Ref)
			o.Changes = x.Changes
		}
	}
	for _, x := range orphans {
		o := getOrCreate(x)
		o.Orphan = true
	}
	for _, x := range deleted {
		o := getOrCreate(x)
		o.Deleted = true
	}

	for ref, o := range m {
		if o.Rendered == nil && o.Applied == nil && !o.Deleted && !o.Orphan {
			// exclude all objects that are clearly not under our control, but got retrieved anyway because some
			// controller passed through the discriminator labels
			delete(m, ref)
		}
	}

	ret := make([]result.ResultObject, 0, len(m))
	for _, o := range m {
		ret = append(ret, *o)
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Ref.GroupVersionKind().String() < ret[j].Ref.GroupVersionKind().String()
	})
	return ret
}
