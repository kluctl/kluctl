package deployment

import (
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/utils"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"strings"
)

func GetIncludedObjectsMetadata(k *k8s.K8sCluster, verbs []string, labels map[string]string, inclusion *utils.Inclusion) ([]*v1.PartialObjectMetadata, error) {
	objects, err := k.ListAllObjectsMetadata(verbs, "", labels)
	if err != nil {
		return nil, err
	}

	var ret []*v1.PartialObjectMetadata

	for _, o := range objects {
		var iv []utils.InclusionEntry
		for t := range getTagsFromObject(o) {
			iv = append(iv, utils.InclusionEntry{
				Type:  "tag",
				Value: t,
			})
		}

		annotations := o.GetAnnotations()
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

func getTagsFromObject(o *v1.PartialObjectMetadata) map[string]bool {
	labels := o.GetLabels()
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
