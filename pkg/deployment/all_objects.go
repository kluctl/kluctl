package deployment

import (
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"strings"
)

func GetIncludedObjectsMetadata(k *k8s.K8sCluster, verbs []string, labels map[string]string, inclusion *utils.Inclusion) ([]*uo.UnstructuredObject, error) {
	objects, err := k.ListAllObjects(verbs, "", labels, true)
	if err != nil {
		return nil, err
	}

	var ret []*uo.UnstructuredObject

	for _, o := range objects {
		var iv []utils.InclusionEntry
		for t := range getTagsFromObject(o) {
			iv = append(iv, utils.InclusionEntry{
				Type:  "tag",
				Value: t,
			})
		}

		annotations := o.GetK8sAnnotations()
		if annotations != nil {
			if itemDir, ok := annotations["kluctl.io/kustomize_dir"]; ok {
				iv = append(iv, utils.InclusionEntry{
					Type:  "deploymentItemDir",
					Value: itemDir,
				})
			}
		}

		if inclusion.CheckIncluded(iv, false) {
			ret = append(ret, o)
		}

	}

	return ret, nil
}

func getTagsFromObject(o *uo.UnstructuredObject) map[string]bool {
	labels := o.GetK8sLabels()
	if len(labels) == 0 {
		return nil
	}

	tags := make(map[string]bool)
	for n, v := range labels {
		if strings.HasPrefix(n, "kluctl.io/tag") {
			tags[v] = true
		}
	}
	return tags
}
