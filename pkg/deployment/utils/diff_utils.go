package utils

import (
	"github.com/codablock/kluctl/pkg/deployment"
	"github.com/codablock/kluctl/pkg/diff"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	k8s2 "github.com/codablock/kluctl/pkg/types/k8s"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"sync"
	"time"
)

type diffUtil struct {
	dew            *DeploymentErrorsAndWarnings
	deployments    []*deployment.DeploymentItem
	appliedObjects map[k8s2.ObjectRef]*uo.UnstructuredObject
	ru             *RemoteObjectUtils

	IgnoreTags        bool
	IgnoreLabels      bool
	IgnoreAnnotations bool

	remoteDiffObjects map[k8s2.ObjectRef]*uo.UnstructuredObject
	NewObjects        []*types.RefAndObject
	ChangedObjects    []*types.ChangedObject
	mutex             sync.Mutex
}

func NewDiffUtil(dew *DeploymentErrorsAndWarnings, deployments []*deployment.DeploymentItem, ru *RemoteObjectUtils, appliedObjects map[k8s2.ObjectRef]*uo.UnstructuredObject) *diffUtil {
	return &diffUtil{
		dew:            dew,
		deployments:    deployments,
		ru:             ru,
		appliedObjects: appliedObjects,
	}
}

func (u *diffUtil) Diff(k *k8s.K8sCluster) {
	var wg sync.WaitGroup

	u.calcRemoteObjectsForDiff()

	for _, d := range u.deployments {
		if !d.CheckInclusionForDeploy() {
			continue
		}

		ignoreForDiffs := d.Project.GetIgnoreForDiffs(u.IgnoreTags, u.IgnoreLabels, u.IgnoreAnnotations)
		for _, o := range d.Objects {
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
		u.NewObjects = append(u.NewObjects, &types.RefAndObject{
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
			u.dew.AddError(lo.GetK8sRef(), err)
			return
		}
		if len(changes) == 0 {
			return
		}

		u.mutex.Lock()
		defer u.mutex.Unlock()
		u.ChangedObjects = append(u.ChangedObjects, &types.ChangedObject{
			Ref:       lo.GetK8sRef(),
			NewObject: ao,
			OldObject: ro,
			Changes:   changes,
		})
	}
}

func (u *diffUtil) calcRemoteObjectsForDiff() {
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

func (u *diffUtil) getRemoteObjectForDiff(localObject *uo.UnstructuredObject) *uo.UnstructuredObject {
	ref := localObject.GetK8sRef()
	diffName := localObject.GetK8sAnnotation("kluctl.io/diff-name")
	if diffName != nil {
		ref.Name = *diffName
	}
	o, _ := u.remoteDiffObjects[ref]
	return o
}
