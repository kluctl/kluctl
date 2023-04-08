package results

import (
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"path"
	"regexp"
	"sort"
	"strings"
)

type ResultStoreSecrets struct {
	k              *k8s.K8sCluster
	writeNamespace string
}

func NewResultStoreSecrets(k *k8s.K8sCluster, writeNamespace string) (*ResultStoreSecrets, error) {
	s := &ResultStoreSecrets{
		k:              k,
		writeNamespace: writeNamespace,
	}

	if s.writeNamespace != "" {
		_, _, err := k.GetSingleObject(k8s2.NewObjectRef("", "v1", "Namespace", s.writeNamespace, ""))
		if err != nil && errors.IsNotFound(err) {
			ns := uo.FromMap(map[string]interface{}{
				"apiVersion": "v1",
				"kind":       "Namespace",
				"metadata": map[string]any{
					"name": s.writeNamespace,
				},
			})
			_, _, err = k.ReadWrite().PatchObject(ns, k8s.PatchOptions{})
			if err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
	}

	return s, nil
}

var invalidChars = regexp.MustCompile(`[^a-zA-Z0-9-]`)

func (s *ResultStoreSecrets) buildName(cr *result.CommandResult) string {
	var name string

	if cr.Project.NormalizedGitUrl != "" {
		s := path.Base(cr.Project.NormalizedGitUrl)
		if s != "" {
			name = s + "-"
		}
	}

	name = strings.ReplaceAll(name, "_", "-")
	name = invalidChars.ReplaceAllString(name, "")

	nameWithId := name + cr.Id
	if len(nameWithId) > 63 {
		trimmedName := []byte(name[:63-len(cr.Id)])
		trimmedName[len(trimmedName)-1] = '-'
		nameWithId = string(trimmedName) + cr.Id
	}

	return nameWithId
}

func (s *ResultStoreSecrets) WriteCommandResult(ctx context.Context, cr *result.CommandResult) error {
	crJson, err := yaml.WriteJsonString(cr.ToReducedObjects())
	if err != nil {
		return err
	}
	compressedCr, err := utils.CompressGzip([]byte(crJson), gzip.BestCompression)
	if err != nil {
		return err
	}

	objectsJson, err := yaml.WriteJsonString(result.CompactedObjects(cr.Objects))
	if err != nil {
		return err
	}
	compressedObjects, err := utils.CompressGzip([]byte(objectsJson), gzip.BestCompression)
	if err != nil {
		return err
	}

	summary := cr.BuildSummary()
	summaryJson, err := yaml.WriteJsonString(summary)
	if err != nil {
		return err
	}

	secret := uo.FromMap(map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      s.buildName(cr),
			"namespace": s.writeNamespace,
			"labels": map[string]any{
				"kluctl.io/result-id": cr.Id,
			},
			"annotations": map[string]any{
				"kluctl.io/result-summary": summaryJson,
			},
		},
		"data": map[string]any{
			"reducedResult":    compressedCr,
			"compactedObjects": compressedObjects,
		},
	})
	if cr.Project.NormalizedGitUrl != "" {
		secret.SetK8sAnnotation("kluctl.io/result-project-normalized-url", cr.Project.NormalizedGitUrl)
	}
	if cr.Project.SubDir != "" {
		secret.SetK8sAnnotation("kluctl.io/result-project-subdir", cr.Project.SubDir)
	}

	_, _, err = s.k.ReadWrite().PatchObject(secret, k8s.PatchOptions{})
	return err
}

func (s *ResultStoreSecrets) ListProjects(ctx context.Context, options ListProjectsOptions) ([]result.ProjectSummary, error) {
	summaries, err := s.ListCommandResultSummaries(ctx, ListCommandResultSummariesOptions{
		ProjectFilter: options.ProjectFilter,
	})
	if err != nil {
		return nil, err
	}

	m := map[result.ProjectKey]result.ProjectSummary{}
	for _, s := range summaries {
		if _, ok := m[s.Project]; !ok {
			m[s.Project] = result.ProjectSummary{Project: s.Project}
		}
	}

	ret := make([]result.ProjectSummary, 0, len(m))
	for _, p := range m {
		for _, rs := range summaries {
			switch rs.Command.Command {
			case "deploy":
				if p.LastDeployCommand == nil {
					rs := rs
					p.LastDeployCommand = &rs
				}
			case "delete":
				if p.LastDeleteCommand == nil {
					rs := rs
					p.LastDeleteCommand = &rs
				}
			case "prune":
				if p.LastPruneCommand == nil {
					rs := rs
					p.LastPruneCommand = &rs
				}
			}
		}
		ret = append(ret, p)
	}

	return ret, nil
}

func (s *ResultStoreSecrets) ListCommandResultSummaries(ctx context.Context, options ListCommandResultSummariesOptions) ([]result.CommandResultSummary, error) {
	l, _, err := s.k.ListMetadata(schema.GroupVersionKind{
		Version: "v1",
		Kind:    "Secret",
	}, "", map[string]string{
		"kluctl.io/result-id": "",
	})
	if err != nil {
		return nil, err
	}

	ret := make([]result.CommandResultSummary, 0, len(l))

	for _, x := range l {
		summaryJson := x.GetK8sAnnotation("kluctl.io/result-summary")
		if summaryJson == nil || *summaryJson == "" {
			continue
		}

		var summary result.CommandResultSummary
		err = yaml.ReadYamlString(*summaryJson, &summary)
		if err != nil {
			continue
		}

		if options.ProjectFilter != nil {
			if summary.Project != *options.ProjectFilter {
				continue
			}
		}

		ret = append(ret, summary)
	}

	sort.Slice(ret, func(i, j int) bool {
		return ret[i].Command.StartTime >= ret[j].Command.StartTime
	})

	return ret, nil
}

func (s *ResultStoreSecrets) GetCommandResult(ctx context.Context, options GetCommandResultOptions) (*result.CommandResult, error) {
	l, _, err := s.k.ListObjects(schema.GroupVersionKind{
		Version: "v1",
		Kind:    "Secret",
	}, "", map[string]string{
		"kluctl.io/result-id": options.Id,
	})
	if err != nil {
		return nil, err
	}
	if len(l) == 0 {
		return nil, nil
	}

	var crJson, objectsJson []byte
	err = utils.RunParallelE(ctx, func() error {
		s, ok, err := l[0].GetNestedString("data", "reducedResult")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("reducedResult field not present for %s", options.Id)
		}
		crJson, err = base64.StdEncoding.DecodeString(s)
		if err != nil {
			return err
		}
		crJson, err = utils.UncompressGzip(crJson)
		if err != nil {
			return err
		}
		return nil
	}, func() error {
		if options.Reduced {
			return nil
		}
		s, ok, err := l[0].GetNestedString("data", "compactedObjects")
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("compactedObjects field not present for %s", options.Id)
		}
		objectsJson, err = base64.StdEncoding.DecodeString(s)
		if err != nil {
			return err
		}

		objectsJson, err = utils.UncompressGzip(objectsJson)
		if err != nil {
			return err
		}
		return err
	})
	if err != nil {
		return nil, err
	}

	var cr result.CommandResult
	err = yaml.ReadYamlBytes(crJson, &cr)
	if err != nil {
		return nil, err
	}
	if !options.Reduced {
		var objects result.CompactedObjects
		err = yaml.ReadYamlBytes(objectsJson, &objects)
		if err != nil {
			return nil, err
		}
		cr.Objects = objects
	}

	return &cr, nil
}
