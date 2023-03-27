package utils

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func objectRefForExclusion(k *k8s.K8sCluster, ref k8s2.ObjectRef) k8s2.ObjectRef {
	ref = k.Resources.FixNamespaceInRef(ref)
	ref.Version = ""
	return ref
}

func isSkipDelete(o *uo.UnstructuredObject) bool {
	if utils.ParseBoolOrFalse(o.GetK8sAnnotation("kluctl.io/skip-delete")) {
		return true
	}

	helmResourcePolicy := o.GetK8sAnnotation("helm.sh/resource-policy")
	if helmResourcePolicy != nil && *helmResourcePolicy == "keep" {
		// warning, this is currently not how Helm handles it. Helm will only respect annotations set in the Chart
		// itself, while Kluctl will also respect it when manually set on the live resource
		// See: https://github.com/helm/helm/issues/8132
		return true
	}

	return false
}

func filterObjectsForDelete(k *k8s.K8sCluster, objects []*uo.UnstructuredObject, apiFilter []string, inclusionHasTags bool, excludedObjects map[k8s2.ObjectRef]bool) ([]*uo.UnstructuredObject, error) {
	filterFunc := func(ar *v1.APIResource) bool {
		if len(apiFilter) == 0 {
			return true
		}
		for _, f := range apiFilter {
			if ar.Name == f || ar.Group == f || ar.Kind == f {
				return true
			}
		}
		return false
	}

	filteredResources := make(map[schema.GroupKind]bool)
	for _, gvk := range k.Resources.GetFilteredPreferredGVKs(filterFunc) {
		filteredResources[gvk.GroupKind()] = true
	}

	var ret []*uo.UnstructuredObject

	for _, o := range objects {
		ref := o.GetK8sRef()
		if _, ok := filteredResources[ref.GroupKind()]; !ok {
			continue
		}

		ownerRefs := o.GetK8sOwnerReferences()
		managedFields := o.GetK8sManagedFields()

		// exclude when explicitly requested
		if isSkipDelete(o) {
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
			mgr, _, _ := mf.GetNestedString("manager")
			if mgr == "kluctl" {
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
			if utils.ParseBoolOrFalse(o.GetK8sAnnotation("kluctl.io/skip-delete-if-tags")) {
				continue
			}
		}

		ret = append(ret, o)
	}
	return ret, nil
}

func FindObjectsForDelete(k *k8s.K8sCluster, allClusterObjects []*uo.UnstructuredObject, inclusionHasTags bool, excludedObjects []k8s2.ObjectRef) ([]k8s2.ObjectRef, error) {
	excludedObjectsMap := make(map[k8s2.ObjectRef]bool)
	for _, ref := range excludedObjects {
		excludedObjectsMap[objectRefForExclusion(k, ref)] = true
	}

	var ret []k8s2.ObjectRef

	for _, filter := range deleteOrder {
		l, err := filterObjectsForDelete(k, allClusterObjects, filter, inclusionHasTags, excludedObjectsMap)
		if err != nil {
			return nil, err
		}
		for _, o := range l {
			ref := o.GetK8sRef()
			excludedObjectsMap[objectRefForExclusion(k, ref)] = true
			ret = append(ret, ref)
		}
	}

	return ret, nil
}

func DeleteObjects(ctx context.Context, k *k8s.K8sCluster, refs []k8s2.ObjectRef, doWait bool) (*result.CommandResult, error) {
	g := utils.NewGoHelper(ctx, 8)

	var ret result.CommandResult
	namespaceNames := make(map[string]bool)
	var mutex sync.Mutex

	handleResult := func(ref k8s2.ObjectRef, apiWarnings []k8s.ApiWarning, err error) {
		mutex.Lock()
		defer mutex.Unlock()

		if err == nil {
			ret.DeletedObjects = append(ret.DeletedObjects, ref)
		} else {
			ret.Errors = append(ret.Errors, result.DeploymentError{
				Ref:     ref,
				Message: err.Error(),
			})
		}
		for _, w := range apiWarnings {
			ret.Warnings = append(ret.Warnings, result.DeploymentError{
				Ref:     ref,
				Message: w.Text,
			})
		}
	}

	for _, ref_ := range refs {
		ref := ref_
		if ref.GroupVersion().String() == "v1" && ref.Kind == "Namespace" {
			namespaceNames[ref.Name] = true
			g.Run(func() {
				apiWarnings, err := k.DeleteSingleObject(ref, k8s.DeleteOptions{NoWait: !doWait, IgnoreNotFoundError: true})
				handleResult(ref, apiWarnings, err)
			})
		}
	}
	g.Wait()

	for _, ref_ := range refs {
		ref := ref_
		if ref.GroupVersion().String() == "v1" && ref.Kind == "Namespace" {
			continue
		}
		if _, ok := namespaceNames[ref.Namespace]; ok {
			// already deleted via namespace
			continue
		}
		g.Run(func() {
			apiWarnings, err := k.DeleteSingleObject(ref, k8s.DeleteOptions{NoWait: !doWait, IgnoreNotFoundError: true})
			handleResult(ref, apiWarnings, err)
		})
	}
	g.Wait()

	return &ret, nil
}
