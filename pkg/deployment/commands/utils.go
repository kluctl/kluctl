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
	remoteDiffNames := map[k8s.ObjectRef]k8s.ObjectRef{}
	appliedDiffNames := map[k8s.ObjectRef]k8s.ObjectRef{}

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
			dn := du.GetDiffRef(x)
			o := getOrCreate(dn)
			o.Rendered = x
		}
	}
	if ru != nil {
		for _, x := range ru.GetFilteredRemoteObjects(nil) {
			dn := du.GetDiffRef(x)
			remoteDiffNames[x.GetK8sRef()] = dn

			o := getOrCreate(dn)
			o.Remote = x
		}
	}

	if au != nil {
		for _, x := range au.GetAppliedObjects() {
			dn := du.GetDiffRef(x)
			appliedDiffNames[x.GetK8sRef()] = dn
			o := getOrCreate(dn)
			o.Applied = x
		}

		for _, x := range au.GetAppliedHookObjects() {
			dn := du.GetDiffRef(x)
			appliedDiffNames[x.GetK8sRef()] = dn
			o := getOrCreate(dn)
			o.Hook = true
		}
		for _, x := range au.GetDeletedObjects() {
			dn, ok := remoteDiffNames[x]
			if !ok {
				dn = x
			}
			o := getOrCreate(dn)
			o.Deleted = true
		}
	}
	if du != nil {
		for _, x := range du.ChangedObjects {
			dn, ok := appliedDiffNames[x.Ref]
			if !ok {
				dn = x.Ref
			}
			o := getOrCreate(dn)
			o.Changes = x.Changes
		}
	}
	if au != nil {
		for _, x := range au.GetNewObjectRefs() {
			dn, ok := appliedDiffNames[x]
			if !ok {
				dn = x
			}
			o := getOrCreate(dn)
			if len(o.Changes) != 0 {
				continue
			}
			o.New = true
		}
	}

	deletedMap := map[k8s.ObjectRef]bool{}
	for _, x := range deleted {
		o := getOrCreate(x)
		o.Deleted = true
		deletedMap[o.Ref] = true
	}
	for _, x := range orphans {
		if _, ok := deletedMap[x]; ok {
			// orphan object also got deleted? This can only mean that deletion did not wait for the object to disappear,
			// so we should treat this object not as orphan anymore
			continue
		}
		o := getOrCreate(x)
		o.Orphan = true

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
