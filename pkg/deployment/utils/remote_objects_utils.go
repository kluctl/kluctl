package utils

import (
	"github.com/kluctl/kluctl/pkg/k8s"
	k8s2 "github.com/kluctl/kluctl/pkg/types/k8s"
	"github.com/kluctl/kluctl/pkg/utils"
	"github.com/kluctl/kluctl/pkg/utils/uo"
	log "github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sync"
)

type RemoteObjectUtils struct {
	dew           *DeploymentErrorsAndWarnings
	remoteObjects map[k8s2.ObjectRef]*uo.UnstructuredObject
	mutex         sync.Mutex
}

func NewRemoteObjectsUtil(dew *DeploymentErrorsAndWarnings) *RemoteObjectUtils {
	return &RemoteObjectUtils{
		dew:           dew,
		remoteObjects: map[k8s2.ObjectRef]*uo.UnstructuredObject{},
	}
}

func (u *RemoteObjectUtils) UpdateRemoteObjects(k *k8s.K8sCluster, labels map[string]string, refs []k8s2.ObjectRef) error {
	if k == nil {
		return nil
	}

	log.Infof("Getting remote objects by commonLabels")
	allObjects, apiWarnings, err := k.ListAllObjects([]string{"get"}, "", labels, false)
	for gvk, aw := range apiWarnings {
		u.dew.AddApiWarnings(k8s2.ObjectRef{GVK: gvk}, aw)
	}
	if err != nil {
		return err
	}

	u.mutex.Lock()
	for _, o := range allObjects {
		u.remoteObjects[o.GetK8sRef()] = o
	}

	notFoundRefsMap := make(map[k8s2.ObjectRef]bool)
	var notFoundRefsList []k8s2.ObjectRef
	for _, ref := range refs {
		if _, ok := u.remoteObjects[ref]; !ok {
			if _, ok = notFoundRefsMap[ref]; !ok {
				notFoundRefsMap[ref] = true
				notFoundRefsList = append(notFoundRefsList, ref)
			}
		}
	}
	u.mutex.Unlock()

	if len(notFoundRefsList) != 0 {
		log.Infof("Getting %d additional remote objects", len(notFoundRefsList))
		r, apiWarnings, err := k.GetObjectsByRefs(notFoundRefsList)
		for ref, aw := range apiWarnings {
			u.dew.AddApiWarnings(ref, aw)
		}
		if err != nil {
			return err
		}
		u.mutex.Lock()
		for _, o := range r {
			u.remoteObjects[o.GetK8sRef()] = o
		}
		u.mutex.Unlock()
	}

	log.Infof("Getting namespaces")
	r, _, err := k.ListObjects(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Namespace",
	}, "", nil)
	if err != nil {
		return err
	}
	for _, o := range r {
		u.remoteObjects[o.GetK8sRef()] = o
	}
	return nil
}

func (u *RemoteObjectUtils) GetRemoteObject(ref k8s2.ObjectRef) *uo.UnstructuredObject {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	o, _ := u.remoteObjects[ref]
	return o
}

func (u *RemoteObjectUtils) ForgetRemoteObject(ref k8s2.ObjectRef) {
	u.mutex.Lock()
	defer u.mutex.Unlock()
	delete(u.remoteObjects, ref)
}

func (u *RemoteObjectUtils) GetFilteredRemoteObjects(inclusion *utils.Inclusion) []*uo.UnstructuredObject {
	var ret []*uo.UnstructuredObject

	u.mutex.Lock()
	defer u.mutex.Unlock()

	for _, o := range u.remoteObjects {
		iv := u.getInclusionEntries(o)
		if inclusion.CheckIncluded(iv, false) {
			ret = append(ret, o)
		}
	}

	return ret
}

func (u *RemoteObjectUtils) getInclusionEntries(o *uo.UnstructuredObject) []utils.InclusionEntry {
	var iv []utils.InclusionEntry
	for _, v := range o.GetK8sLabelsWithRegex("^kluctl.io/tag-\\d+$") {
		iv = append(iv, utils.InclusionEntry{
			Type:  "tag",
			Value: v,
		})
	}

	if itemDir := o.GetK8sAnnotation("kluctl.io/kustomize_dir"); itemDir != nil {
		iv = append(iv, utils.InclusionEntry{
			Type:  "deploymentItemDir",
			Value: *itemDir,
		})
	}
	return iv
}
