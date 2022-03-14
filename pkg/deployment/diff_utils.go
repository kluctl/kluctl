package deployment

import (
	"github.com/codablock/kluctl/pkg/diff"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	k8s2 "github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"sync"
	"time"
)

type diffUtil struct {
	dew            *deploymentErrorsAndWarnings
	deployments    []*deploymentItem
	appliedObjects map[k8s2.ObjectRef]*uo.UnstructuredObject
	remoteObjects  map[k8s2.ObjectRef]*uo.UnstructuredObject

	ignoreTags        bool
	ignoreLabels      bool
	ignoreAnnotations bool

	remoteDiffObjects map[k8s2.ObjectRef]*uo.UnstructuredObject
	newObjects        []*types.RefAndObject
	changedObjects    []*types.ChangedObject
	mutex             sync.Mutex
}

func NewDiffUtil(dew *deploymentErrorsAndWarnings, deployments []*deploymentItem, remoteObjects map[k8s2.ObjectRef]*uo.UnstructuredObject, appliedObjects map[k8s2.ObjectRef]*uo.UnstructuredObject) *diffUtil {
	return &diffUtil{
		dew:            dew,
		deployments:    deployments,
		remoteObjects:  remoteObjects,
		appliedObjects: appliedObjects,
	}
}

func (u *diffUtil) diff(k *k8s.K8sCluster) {
	var wg sync.WaitGroup

	u.calcRemoteObjectsForDiff()

	for _, d := range u.deployments {
		if !d.checkInclusionForDeploy() {
			continue
		}

		ignoreForDiffs := d.project.getIgnoreForDiffs(u.ignoreTags, u.ignoreLabels, u.ignoreAnnotations)
		for _, o := range d.objects {
			o := o
			ref := o.GetK8sRef()
			ao, ok := u.appliedObjects[ref]
			if !ok {
				// if we can't even find it in appliedObjects, it probably ran into an error
				continue
			}
			ro := u.getRemoteObjectForDiff(o)

			wg.Add(1)
			go func() {
				defer wg.Done()
				u.diffObject(k, o, ao, ro, ignoreForDiffs)
			}()
		}
	}
	wg.Wait()
}

func (u *diffUtil) diffObject(k *k8s.K8sCluster, lo *uo.UnstructuredObject, ao *uo.UnstructuredObject, ro *uo.UnstructuredObject, ignoreForDiffs []*types.IgnoreForDiffItemConfig) {
	if ao != nil && ro == nil {
		u.mutex.Lock()
		defer u.mutex.Unlock()
		u.newObjects = append(u.newObjects, &types.RefAndObject{
			Ref:    ao.GetK8sRef(),
			Object: ao,
		})
	} else if ao == nil && ro != nil {
		// deleted?
		return
	} else if ao == nil && ro == nil {
		// did not apply? (e.g. in downscale command)
		return
	} else {
		nao := diff.NormalizeObject(k, ao, ignoreForDiffs, lo)
		nro := diff.NormalizeObject(k, ro, ignoreForDiffs, lo)
		changes, err := diff.Diff(nro, nao)
		if err != nil {
			u.dew.addError(lo.GetK8sRef(), err)
			return
		}
		if len(changes) == 0 {
			return
		}

		u.mutex.Lock()
		defer u.mutex.Unlock()
		u.changedObjects = append(u.changedObjects, &types.ChangedObject{
			Ref:       lo.GetK8sRef(),
			NewObject: ao,
			OldObject: ro,
			Changes:   changes,
		})
	}
}

func (u *diffUtil) calcRemoteObjectsForDiff() {
	u.remoteDiffObjects = make(map[k8s2.ObjectRef]*uo.UnstructuredObject)
	for _, o := range u.remoteObjects {
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

func (u *diffUtil) getRemoteObjectForDiff(localObject *uo.UnstructuredObject) *uo.UnstructuredObject {
	ref := localObject.GetK8sRef()
	diffName := localObject.GetK8sAnnotation("kluctl.io/diff-name")
	if diffName != nil {
		ref.Name = *diffName
	}
	o, _ := u.remoteDiffObjects[ref]
	return o
}
