package results

import (
	"compress/gzip"
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	toolscache "k8s.io/client-go/tools/cache"
	"path"
	"regexp"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sort"
	"strings"
	"sync"
)

type ResultStoreSecrets struct {
	ctx    context.Context
	client client.Client
	cache  cache.Cache
	reader client.Reader

	writeNamespace   string
	keepResultsCount int

	mutex    sync.Mutex
	informer cache.Informer
}

func NewResultStoreSecrets(ctx context.Context, client client.Client, cache cache.Cache, writeNamespace string, keepResultsCount int) (*ResultStoreSecrets, error) {
	s := &ResultStoreSecrets{
		ctx:              ctx,
		client:           client,
		cache:            cache,
		writeNamespace:   writeNamespace,
		keepResultsCount: keepResultsCount,
	}
	if cache != nil {
		s.reader = cache
	} else {
		s.reader = client
	}
	return s, nil
}

func (s *ResultStoreSecrets) ensureInformer() error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if s.informer != nil {
		return nil
	}
	if s.cache == nil {
		return nil
	}

	var partialSecret metav1.PartialObjectMetadata
	partialSecret.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "Secret"})

	informer, err := s.cache.GetInformer(s.ctx, &partialSecret)
	if err != nil {
		return err
	}

	s.informer = informer
	s.cache.WaitForCacheSync(s.ctx)

	return nil
}

var invalidChars = regexp.MustCompile(`[^a-zA-Z0-9-]`)

func (s *ResultStoreSecrets) buildName(cr *result.CommandResult) string {
	var name string

	if cr.ProjectKey.GitRepoKey.Path != "" {
		s := path.Base(cr.ProjectKey.GitRepoKey.Path)
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

func (s *ResultStoreSecrets) ensureWriteNamespace() error {
	if s.writeNamespace == "" {
		return fmt.Errorf("missing writeNamespace")
	}
	var ns corev1.Namespace
	err := s.reader.Get(s.ctx, client.ObjectKey{Name: s.writeNamespace}, &ns)
	if err != nil && errors.IsNotFound(err) {
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: s.writeNamespace,
			},
		}
		err = s.client.Create(s.ctx, ns)
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}

func (s *ResultStoreSecrets) WriteCommandResult(cr *result.CommandResult) error {
	err := s.ensureInformer()
	if err != nil {
		return err
	}

	err = s.ensureWriteNamespace()
	if err != nil {
		return err
	}

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

	secret := corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.buildName(cr),
			Namespace: s.writeNamespace,
			Labels: map[string]string{
				"kluctl.io/result-id": cr.Id,
			},
			Annotations: map[string]string{
				"kluctl.io/result-summary": summaryJson,
			},
		},
		Data: map[string][]byte{
			"reducedResult":    compressedCr,
			"compactedObjects": compressedObjects,
		},
	}
	if cr.ProjectKey.GitRepoKey.String() != "" {
		secret.Annotations["kluctl.io/result-project-repo-key"] = cr.ProjectKey.GitRepoKey.String()
	}
	if cr.ProjectKey.SubDir != "" {
		secret.Annotations["kluctl.io/result-project-subdir"] = cr.ProjectKey.SubDir
	}

	err = s.client.Patch(s.ctx, &secret, client.Apply, client.FieldOwner("kluctl-results"))
	if err != nil {
		return err
	}

	err = s.cleanupResults(cr.ProjectKey, cr.TargetKey)
	if err != nil {
		return err
	}

	return nil
}

func (s *ResultStoreSecrets) cleanupResults(project result.ProjectKey, target result.TargetKey) error {
	results, err := s.ListCommandResultSummaries(ListCommandResultSummariesOptions{
		ProjectFilter: &project,
	})
	if err != nil {
		return err
	}

	cnt := 0
	for _, rs := range results {
		rs := rs

		if rs.TargetKey != target {
			continue
		}
		cnt++

		if cnt > s.keepResultsCount {
			err := s.client.DeleteAllOf(s.ctx, &corev1.Secret{}, client.InNamespace(s.writeNamespace), client.MatchingLabels{
				"kluctl.io/result-id": rs.Id,
			})
			if err != nil {
				status.Warningf(s.ctx, "Failed to delete old command result %s: %s", rs.Id, err)
			} else {
				status.Infof(s.ctx, "Deleted old command result %s", rs.Id)
			}
		}
	}
	return nil
}

func (s *ResultStoreSecrets) ListCommandResultSummaries(options ListCommandResultSummariesOptions) ([]result.CommandResultSummary, error) {
	err := s.ensureInformer()
	if err != nil {
		return nil, err
	}

	var l metav1.PartialObjectMetadataList
	l.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "SecretList"})
	err = s.reader.List(s.ctx, &l, client.HasLabels{"kluctl.io/result-id"})
	if err != nil {
		return nil, err
	}

	ret := make([]result.CommandResultSummary, 0, len(l.Items))

	for _, x := range l.Items {
		summary, err := s.parseSummary(&x)
		if err != nil {
			continue
		}
		if !s.filterSummary(summary, options.ProjectFilter) {
			continue
		}

		ret = append(ret, *summary)
	}

	sort.Slice(ret, func(i, j int) bool {
		return lessSummary(&ret[i], &ret[j])
	})

	return ret, nil
}

func (s *ResultStoreSecrets) parseSummary(obj client.Object) (*result.CommandResultSummary, error) {
	if len(obj.GetAnnotations()) == 0 {
		return nil, nil
	}
	summaryJson := obj.GetAnnotations()["kluctl.io/result-summary"]
	if summaryJson == "" {
		return nil, nil
	}

	var summary result.CommandResultSummary
	err := yaml.ReadYamlString(summaryJson, &summary)
	if err != nil {
		return nil, err
	}

	return &summary, nil
}

func (s *ResultStoreSecrets) filterSummary(summary *result.CommandResultSummary, project *result.ProjectKey) bool {
	if project != nil {
		if project.GitRepoKey.String() != "" && summary.ProjectKey.GitRepoKey != project.GitRepoKey {
			return false
		}
		if project.SubDir != "" && summary.ProjectKey.SubDir != project.SubDir {
			return false
		}
	}

	return true
}

func (s *ResultStoreSecrets) WatchCommandResultSummaries(options ListCommandResultSummariesOptions, update func(summary *result.CommandResultSummary), delete func(id string)) (func(), error) {
	err := s.ensureInformer()
	if err != nil {
		return nil, err
	}

	handler := func(obj any, deleted bool) {
		obj2, ok := obj.(client.Object)
		if !ok {
			return
		}
		summary, err := s.parseSummary(obj2)
		if err != nil {
			return
		}
		if summary == nil {
			return
		}
		if !s.filterSummary(summary, options.ProjectFilter) {
			return
		}
		if deleted {
			delete(summary.Id)
		} else {
			update(summary)
		}
	}

	r, err := s.informer.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			handler(obj, false)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			handler(newObj, false)
		},
		DeleteFunc: func(obj interface{}) {
			handler(obj, true)
		},
	})
	if err != nil {
		return nil, err
	}

	cancel := func() {
		_ = s.informer.RemoveEventHandler(r)
	}
	return cancel, nil
}

func (s *ResultStoreSecrets) getCommandResultSecret(id string) (*metav1.PartialObjectMetadata, error) {
	err := s.ensureInformer()
	if err != nil {
		return nil, err
	}

	var l metav1.PartialObjectMetadataList
	l.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "SecretList"})
	err = s.reader.List(s.ctx, &l, client.MatchingLabels{"kluctl.io/result-id": id})
	if err != nil {
		return nil, err
	}
	if len(l.Items) == 0 {
		return nil, nil
	}
	return &l.Items[0], nil
}

func (s *ResultStoreSecrets) HasCommandResult(id string) (bool, error) {
	secret, err := s.getCommandResultSecret(id)
	if err != nil {
		return false, err
	}
	return secret != nil, nil
}

func (s *ResultStoreSecrets) GetCommandResultSummary(id string) (*result.CommandResultSummary, error) {
	secret, err := s.getCommandResultSecret(id)
	if err != nil {
		return nil, err
	}
	return s.parseSummary(secret)
}

func (s *ResultStoreSecrets) GetCommandResult(options GetCommandResultOptions) (*result.CommandResult, error) {
	has, err := s.HasCommandResult(options.Id)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, nil
	}

	var l corev1.SecretList
	err = s.reader.List(s.ctx, &l, client.MatchingLabels{
		"kluctl.io/result-id": options.Id,
	})
	if err != nil {
		return nil, err
	}
	if len(l.Items) == 0 {
		return nil, nil
	}

	secret := l.Items[0]

	var crJson, objectsJson []byte
	err = utils.RunParallelE(s.ctx, func() error {
		j, ok := secret.Data["reducedResult"]
		if !ok {
			return fmt.Errorf("reducedResult field not present for %s", options.Id)
		}
		j, err := utils.UncompressGzip(j)
		if err != nil {
			return err
		}
		crJson = j
		return nil
	}, func() error {
		if options.Reduced {
			return nil
		}
		j, ok := secret.Data["compactedObjects"]
		if !ok {
			return fmt.Errorf("compactedObjects field not present for %s", options.Id)
		}
		j, err := utils.UncompressGzip(j)
		if err != nil {
			return err
		}
		objectsJson = j
		return nil
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
