package k8s

import (
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"strconv"
	"sync"
)

// either names or apigroups
var deleteOrder = [][]string{
	// delete namespaces first
	{
		"Namespace",
	},
	// high level stuff from CRDs
	{
		"monitoring.coreos.com",
		"kafka.strimzi.io",
		"zookeeper.pravega.io",
		"elasticsearch.k8s.elastic.co",
		"cert-manager.io",
		"bitnami.com",
		"acid.zalan.do",
	},
	{
		// generic high level stuff
		"Deployment",
		"StatefulSet",
		"DaemonSet",
		"Service",
		"Ingress",
	},
	// and now everything else
	nil,
}

func objectRefForExclusion(k *K8sCluster, ref types.ObjectRef) types.ObjectRef {
	ref = k.RemoveNamespaceFromRefIfNeeded(ref)
	ref.GVK.Version = ""
	return ref
}

func filterObjectsForDelete(k *K8sCluster, objects []*v1.PartialObjectMetadata, apiFilter []string, inclusion *utils.Inclusion, excludedObjects map[types.ObjectRef]bool) ([]*v1.PartialObjectMetadata, error) {
	filteredResources := make(map[schema.GroupKind]bool)
	for _, gk := range k.GetFilteredGKs(apiFilter) {
		filteredResources[gk] = true
	}

	inclusionHasTags := inclusion.HasType("tags")
	var ret []*v1.PartialObjectMetadata

	for _, o := range objects {
		ref := types.RefFromPartialObject(o)
		if _, ok := filteredResources[ref.GVK.GroupKind()]; !ok {
			continue
		}

		annotations := o.GetAnnotations()
		ownerRefs := o.GetOwnerReferences()
		managedFields := o.GetManagedFields()

		// exclude when explicitly requested
		skipDelete, err := strconv.ParseBool(annotations["kluctl.io/skip-delete"])
		if err == nil && skipDelete {
			continue
		}

		// exclude objects which are owned by some other object
		if len(ownerRefs) != 0 {
			continue
		}

		if len(managedFields) == 0 {
			// We don't know who manages it...be safe and exclude it
			continue
		}

		// check if kluctl is managing this object
		found := false
		for _, mf := range managedFields {
			if mf.Manager == "kluctl" {
				found = true
				break
			}
		}
		if !found {
			// This object is not managed by kluctl, so we shouldn't delete it
			continue
		}

		// exclude objects from excluded_objects
		if _, ok := excludedObjects[objectRefForExclusion(k, ref)]; ok {
			continue
		}

		// exclude resources which have the 'kluctl.io/skip-delete-if-tags' annotation set
		if inclusionHasTags {
			skipDeleteIfTags, err := strconv.ParseBool(annotations["kluctl.io/skip-delete-if-tags"])
			if err == nil && skipDeleteIfTags {
				continue
			}
		}

		ret = append(ret, o)
	}
	return ret, nil
}

func FindObjectsForDelete(k *K8sCluster, labels map[string]string, inclusion *utils.Inclusion, excludedObjects []types.ObjectRef) ([]types.ObjectRef, error) {
	log.Infof("Getting all cluster objects matching deleteByLabels")

	excludedObjectsMap := make(map[types.ObjectRef]bool)
	for _, ref := range excludedObjects {
		excludedObjectsMap[objectRefForExclusion(k, ref)] = true
	}

	allClusterObjects, err := GetIncludedObjectsMetadata(k, []string{"delete"}, labels, inclusion)
	if err != nil {
		return nil, err
	}

	var ret []types.ObjectRef

	for _, filter := range deleteOrder {
		l, err := filterObjectsForDelete(k, allClusterObjects, filter, inclusion, excludedObjectsMap)
		if err != nil {
			return nil, err
		}
		for _, o := range l {
			ref := types.RefFromPartialObject(o)
			excludedObjectsMap[objectRefForExclusion(k, ref)] = true
			ret = append(ret, ref)
		}
	}

	return ret, nil
}

func DeleteObjects(k *K8sCluster, refs []types.ObjectRef, doWait bool) (*types.CommandResult, error) {
	wp := utils.NewDebuggerAwareWorkerPool(8)
	defer wp.StopWait(false)

	namespaceNames := make(map[string]bool)
	var deletedObjects []types.ObjectRef
	var mutex sync.Mutex

	for _, ref_ := range refs {
		ref := ref_
		if ref.GVK.GroupVersion().String() == "v1" && ref.GVK.Kind == "Namespace" {
			namespaceNames[ref.Name] = true
			wp.Submit(func() error {
				err := k.DeleteSingleObject(ref, DeleteOptions{NoWait: !doWait})
				mutex.Lock()
				defer mutex.Unlock()
				if err == nil {
					deletedObjects = append(deletedObjects, ref)
				}
				return err
			})
		}
	}
	err := wp.StopWait(true)
	if err != nil {
		return nil, err
	}

	for _, ref_ := range refs {
		ref := ref_
		if ref.GVK.GroupVersion().String() == "v1" && ref.GVK.Kind == "Namespace" {
			continue
		}
		if _, ok := namespaceNames[ref.Namespace]; ok {
			// already deleted via namespace
			continue
		}
		wp.Submit(func() error {
			err := k.DeleteSingleObject(ref, DeleteOptions{NoWait: !doWait})
			mutex.Lock()
			defer mutex.Unlock()
			if err == nil {
				deletedObjects = append(deletedObjects, ref)
			}
			return err
		})
	}
	err = wp.StopWait(false)
	return &types.CommandResult{DeletedObjects: deletedObjects}, err
}
