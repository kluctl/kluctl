package utils

import (
	"github.com/kluctl/kluctl/v2/pkg/deployment"
	"github.com/kluctl/kluctl/v2/pkg/diff"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"sort"
	"sync"
	"time"
)

type DiffUtil struct {
	dew            *DeploymentErrorsAndWarnings
	appliedObjects map[k8s2.ObjectRef]*uo.UnstructuredObject
	ru             *RemoteObjectUtils

	IgnoreTags        bool
	IgnoreLabels      bool
	IgnoreAnnotations bool
	Swapped           bool

	remoteDiffObjects map[k8s2.ObjectRef]*uo.UnstructuredObject
	ChangedObjects    []result.ChangedObject
	mutex             sync.Mutex
}

func NewDiffUtil(dew *DeploymentErrorsAndWarnings, ru *RemoteObjectUtils, appliedObjects map[k8s2.ObjectRef]*uo.UnstructuredObject) *DiffUtil {
	u := &DiffUtil{
		dew:            dew,
		ru:             ru,
		appliedObjects: appliedObjects,
	}
	u.calcRemoteObjectsForDiff()
	return u
}

func (u *DiffUtil) DiffDeploymentItems(deployments []*deployment.DeploymentItem) {
	var wg sync.WaitGroup

	for _, d := range deployments {
		ignoreForDiffs := d.Project.GetIgnoreForDiffs(u.IgnoreTags, u.IgnoreLabels, u.IgnoreAnnotations)
		u.diffObjects(d.Objects, ignoreForDiffs, &wg)
	}
	wg.Wait()

	u.sortChanges()
}

func (u *DiffUtil) DiffObjects(objects []*uo.UnstructuredObject) {
	var wg sync.WaitGroup
	u.diffObjects(objects, nil, &wg)
	wg.Wait()
	u.sortChanges()
}

func (u *DiffUtil) sortChanges() {
	sort.Slice(u.ChangedObjects, func(i, j int) bool {
		return u.ChangedObjects[i].Ref.String() < u.ChangedObjects[j].Ref.String()
	})
}

func (u *DiffUtil) diffObjects(objects []*uo.UnstructuredObject, ignoreForDiffs []*types.IgnoreForDiffItemConfig, wg *sync.WaitGroup) {
	for _, o := range objects {
		o := o
		ref := o.GetK8sRef()
		ao, ok := u.appliedObjects[ref]
		if !ok {
			// if we can't even find it in appliedObjects, it probably ran into an error
			continue
		}
		diffRef, ro := u.getRemoteObjectForDiff(o)

		wg.Add(1)
		go func() {
			defer wg.Done()
			if u.Swapped {
				u.diffObject(o, diffRef, ro, ao, ignoreForDiffs)
			} else {
				u.diffObject(o, diffRef, ao, ro, ignoreForDiffs)
			}
		}()
	}
}

func (u *DiffUtil) diffObject(lo *uo.UnstructuredObject, diffRef k8s2.ObjectRef, ao *uo.UnstructuredObject, ro *uo.UnstructuredObject, ignoreForDiffs []*types.IgnoreForDiffItemConfig) {
	if ao != nil && ro == nil {
		// new?
		return
	} else if ao == nil && ro != nil {
		// deleted?
		return
	} else if ao == nil && ro == nil {
		// did not apply? (e.g. in downscale command)
		return
	} else {
		nao, err := diff.NormalizeObject(ao, ignoreForDiffs, lo)
		if err != nil {
			u.dew.AddError(lo.GetK8sRef(), err)
			return
		}
		nro, err := diff.NormalizeObject(ro, ignoreForDiffs, lo)
		if err != nil {
			u.dew.AddError(lo.GetK8sRef(), err)
			return
		}
		changes, err := diff.Diff(nro, nao)
		if err != nil {
			u.dew.AddError(lo.GetK8sRef(), err)
			return
		}
		if len(changes) == 0 {
			return
		}

		u.mutex.Lock()
		defer u.mutex.Unlock()
		u.ChangedObjects = append(u.ChangedObjects, result.ChangedObject{
			Ref:     diffRef,
			Changes: changes,
		})
	}
}

func (u *DiffUtil) calcRemoteObjectsForDiff() {
	u.remoteDiffObjects = make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	for _, o := range u.ru.remoteObjects {
		diffName := o.GetK8sAnnotation("kluctl.io/diff-name")
		if diffName == nil {
			x := o.GetK8sName()
			diffName = &x
		}
		diffRef := o.GetK8sRef()
		diffRef.Name = *diffName
		oldCreationTime := time.Time{}
		if old, ok := u.remoteDiffObjects[diffRef]; ok {
			oldCreationTime = old.GetK8sCreationTime()
		}
		if oldCreationTime.IsZero() || o.GetK8sCreationTime().After(oldCreationTime) {
			u.remoteDiffObjects[diffRef] = o
		}
	}
}

func (u *DiffUtil) getRemoteObjectForDiff(localObject *uo.UnstructuredObject) (k8s2.ObjectRef, *uo.UnstructuredObject) {
	ref := localObject.GetK8sRef()
	diffName := localObject.GetK8sAnnotation("kluctl.io/diff-name")
	if diffName != nil {
		ref.Name = *diffName
	}
	o, _ := u.remoteDiffObjects[ref]
	return ref, o
}
