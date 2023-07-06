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
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/rest"
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
	ctx context.Context

	client               client.Client
	commandResultsCache  cache.Cache
	validateResultsCache cache.Cache

	writeNamespace           string
	keepCommandResultsCount  int
	keepValidateResultsCount int

	mutex sync.Mutex
}

func NewResultStoreSecrets(ctx context.Context, config *rest.Config, client client.Client, writeNamespace string, keepCommandResultsCount int, keepValidateResultsCount int) (*ResultStoreSecrets, error) {
	req1, _ := labels.NewRequirement("kluctl.io/command-result-id", selection.Exists, nil)
	req2, _ := labels.NewRequirement("kluctl.io/validate-result-id", selection.Exists, nil)

	c1, err := cache.New(config, cache.Options{
		Mapper:               client.RESTMapper(),
		DefaultLabelSelector: labels.NewSelector().Add(*req1),
	})
	if err != nil {
		return nil, err
	}
	c2, err := cache.New(config, cache.Options{
		Mapper:               client.RESTMapper(),
		DefaultLabelSelector: labels.NewSelector().Add(*req2),
	})
	if err != nil {
		return nil, err
	}

	go c1.Start(ctx)
	go c2.Start(ctx)

	s := &ResultStoreSecrets{
		ctx:                      ctx,
		client:                   client,
		writeNamespace:           writeNamespace,
		commandResultsCache:      c1,
		validateResultsCache:     c2,
		keepCommandResultsCount:  keepCommandResultsCount,
		keepValidateResultsCount: keepValidateResultsCount,
	}
	return s, nil
}

var invalidChars = regexp.MustCompile(`[^a-zA-Z0-9-]`)

func (s *ResultStoreSecrets) buildName(prefix string, id string, projectKey result.ProjectKey) string {
	name := ""

	if projectKey.GitRepoKey.Path != "" {
		name = path.Base(projectKey.GitRepoKey.Path)
	}

	name = strings.ReplaceAll(name, "_", "-")
	name = invalidChars.ReplaceAllString(name, "")

	maxNameLen := 63
	maxNameLen -= len(id) + 1     // +1 for '-'
	maxNameLen -= len(prefix) + 1 // +1 for '-'
	if len(name) > maxNameLen {
		name = name[:maxNameLen]
	}

	return prefix + "-" + name + "-" + id
}

func (s *ResultStoreSecrets) ensureWriteNamespace() error {
	if s.writeNamespace == "" {
		return fmt.Errorf("missing writeNamespace")
	}
	var ns corev1.Namespace
	err := s.client.Get(s.ctx, client.ObjectKey{Name: s.writeNamespace}, &ns)
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
	err := s.ensureWriteNamespace()
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
			Name:      s.buildName("cr", cr.Id, cr.ProjectKey),
			Namespace: s.writeNamespace,
			Labels: map[string]string{
				"kluctl.io/command-result-id": cr.Id,
			},
			Annotations: map[string]string{
				"kluctl.io/command-result-summary": summaryJson,
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

	err = s.cleanupCommandResults(cr.ProjectKey, cr.TargetKey)
	if err != nil {
		return err
	}

	return nil
}

func (s *ResultStoreSecrets) WriteValidateResult(vr *result.ValidateResult) error {
	err := s.ensureWriteNamespace()
	if err != nil {
		return err
	}

	vrJson, err := yaml.WriteJsonString(vr)
	if err != nil {
		return err
	}
	compressedVr, err := utils.CompressGzip([]byte(vrJson), gzip.BestCompression)
	if err != nil {
		return err
	}

	summary := vr.BuildSummary()
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
			Name:      s.buildName("vr", vr.Id, vr.ProjectKey),
			Namespace: s.writeNamespace,
			Labels: map[string]string{
				"kluctl.io/validate-result-id": vr.Id,
			},
			Annotations: map[string]string{
				"kluctl.io/validate-result-summary": summaryJson,
			},
		},
		Data: map[string][]byte{
			"result": compressedVr,
		},
	}
	if vr.ProjectKey.GitRepoKey.String() != "" {
		secret.Annotations["kluctl.io/result-project-repo-key"] = vr.ProjectKey.GitRepoKey.String()
	}
	if vr.ProjectKey.SubDir != "" {
		secret.Annotations["kluctl.io/result-project-subdir"] = vr.ProjectKey.SubDir
	}

	err = s.client.Patch(s.ctx, &secret, client.Apply, client.FieldOwner("kluctl-results"))
	if err != nil {
		return err
	}

	err = s.cleanupValidateResults(vr.ProjectKey, vr.TargetKey)
	if err != nil {
		return err
	}

	return nil
}

func (s *ResultStoreSecrets) cleanupCommandResults(project result.ProjectKey, target result.TargetKey) error {
	results, err := s.ListCommandResultSummaries(ListResultSummariesOptions{
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

		if cnt > s.keepCommandResultsCount {
			err := s.client.DeleteAllOf(s.ctx, &corev1.Secret{}, client.InNamespace(s.writeNamespace), client.MatchingLabels{
				"kluctl.io/command-result-id": rs.Id,
			})
			if err != nil {
				status.Warning(s.ctx, "Failed to delete old command result %s: %s", rs.Id, err)
			} else {
				status.Info(s.ctx, "Deleted old command result %s", rs.Id)
			}
		}
	}
	return nil
}

func (s *ResultStoreSecrets) cleanupValidateResults(project result.ProjectKey, target result.TargetKey) error {
	results, err := s.ListValidateResultSummaries(ListResultSummariesOptions{
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

		if cnt > s.keepValidateResultsCount {
			err := s.client.DeleteAllOf(s.ctx, &corev1.Secret{}, client.InNamespace(s.writeNamespace), client.MatchingLabels{
				"kluctl.io/validate-result-id": rs.Id,
			})
			if err != nil {
				status.Warning(s.ctx, "Failed to delete old validate result %s: %s", rs.Id, err)
			} else {
				status.Info(s.ctx, "Deleted old validate result %s", rs.Id)
			}
		}
	}
	return nil
}

func (s *ResultStoreSecrets) ListCommandResultSummaries(options ListResultSummariesOptions) ([]result.CommandResultSummary, error) {
	var l metav1.PartialObjectMetadataList
	l.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "SecretList"})
	err := s.commandResultsCache.List(s.ctx, &l, client.HasLabels{"kluctl.io/command-result-id"})
	if err != nil {
		return nil, err
	}

	ret := make([]result.CommandResultSummary, 0, len(l.Items))

	for _, x := range l.Items {
		summary, err := s.parseCommandSummary(x.GetAnnotations())
		if err != nil {
			continue
		}
		if !FilterProject(summary.ProjectKey, options.ProjectFilter) {
			continue
		}

		ret = append(ret, *summary)
	}

	sort.Slice(ret, func(i, j int) bool {
		return lessCommandSummary(&ret[i], &ret[j])
	})

	return ret, nil
}

func (s *ResultStoreSecrets) parseCommandSummary(a map[string]string) (*result.CommandResultSummary, error) {
	if len(a) == 0 {
		return nil, nil
	}
	summaryJson := a["kluctl.io/command-result-summary"]
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

func (s *ResultStoreSecrets) parseValidateSummary(a map[string]string) (*result.ValidateResultSummary, error) {
	if len(a) == 0 {
		return nil, nil
	}
	summaryJson := a["kluctl.io/validate-result-summary"]
	if summaryJson == "" {
		return nil, nil
	}

	var summary result.ValidateResultSummary
	err := yaml.ReadYamlString(summaryJson, &summary)
	if err != nil {
		return nil, err
	}

	return &summary, nil
}

func (s *ResultStoreSecrets) WatchCommandResultSummaries(options ListResultSummariesOptions) ([]*result.CommandResultSummary, <-chan WatchCommandResultSummaryEvent, context.CancelFunc, error) {
	ch := make(chan WatchCommandResultSummaryEvent)

	buildEvent := func(obj any) *WatchCommandResultSummaryEvent {
		var o metav1.Object
		switch o2 := obj.(type) {
		case metav1.Object:
			o = o2
		case toolscache.DeletedFinalStateUnknown:
			o = o2.Obj.(metav1.Object)
		}
		if o == nil {
			return nil
		}
		summary, err := s.parseCommandSummary(o.GetAnnotations())
		if err != nil {
			return nil
		}
		if !FilterProject(summary.ProjectKey, options.ProjectFilter) {
			return nil
		}
		return &WatchCommandResultSummaryEvent{
			Summary: summary,
		}
	}

	doUpdate := func(obj any) {
		e := buildEvent(obj)
		if e == nil {
			return
		}
		ch <- *e
	}
	doDelete := func(obj any) {
		e := buildEvent(obj)
		if e == nil {
			return
		}
		e.Delete = true
		ch <- *e
	}

	inf, err := s.commandResultsCache.GetInformer(s.ctx, &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
	})
	if err != nil {
		return nil, nil, nil, err
	}

	handle, err := inf.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			doUpdate(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			doUpdate(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			doDelete(obj)
		},
	})
	if err != nil {
		return nil, nil, nil, err
	}

	cancel := func() {
		_ = inf.RemoveEventHandler(handle)
	}
	return nil, ch, cancel, nil
}

func (s *ResultStoreSecrets) GetCommandResult(options GetCommandResultOptions) (*result.CommandResult, error) {
	var l corev1.SecretList
	err := s.client.List(s.ctx, &l, client.MatchingLabels{
		"kluctl.io/command-result-id": options.Id,
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

func (s *ResultStoreSecrets) ListValidateResultSummaries(options ListResultSummariesOptions) ([]result.ValidateResultSummary, error) {
	var l metav1.PartialObjectMetadataList
	l.SetGroupVersionKind(schema.GroupVersionKind{Version: "v1", Kind: "SecretList"})
	err := s.validateResultsCache.List(s.ctx, &l, client.HasLabels{"kluctl.io/validate-result-id"})
	if err != nil {
		return nil, err
	}

	ret := make([]result.ValidateResultSummary, 0, len(l.Items))

	for _, x := range l.Items {
		summary, err := s.parseValidateSummary(x.GetAnnotations())
		if err != nil {
			continue
		}
		if !FilterProject(summary.ProjectKey, options.ProjectFilter) {
			continue
		}

		ret = append(ret, *summary)
	}

	sort.Slice(ret, func(i, j int) bool {
		return lessValidateSummary(&ret[i], &ret[j])
	})

	return ret, nil
}

func (s *ResultStoreSecrets) WatchValidateResultSummaries(options ListResultSummariesOptions) ([]*result.ValidateResultSummary, <-chan WatchValidateResultSummaryEvent, context.CancelFunc, error) {
	ch := make(chan WatchValidateResultSummaryEvent)

	buildEvent := func(obj any) *WatchValidateResultSummaryEvent {
		var o metav1.Object
		switch o2 := obj.(type) {
		case metav1.Object:
			o = o2
		case toolscache.DeletedFinalStateUnknown:
			o = o2.Obj.(metav1.Object)
		}
		if o == nil {
			return nil
		}
		summary, err := s.parseValidateSummary(o.GetAnnotations())
		if err != nil {
			return nil
		}
		if !FilterProject(summary.ProjectKey, options.ProjectFilter) {
			return nil
		}
		return &WatchValidateResultSummaryEvent{
			Summary: summary,
		}
	}

	doUpdate := func(obj any) {
		e := buildEvent(obj)
		if e == nil {
			return
		}
		ch <- *e
	}
	doDelete := func(obj any) {
		e := buildEvent(obj)
		if e == nil {
			return
		}
		e.Delete = true
		ch <- *e
	}

	inf, err := s.validateResultsCache.GetInformer(s.ctx, &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		},
	})
	if err != nil {
		return nil, nil, nil, err
	}

	handle, err := inf.AddEventHandler(toolscache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			doUpdate(obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			doUpdate(newObj)
		},
		DeleteFunc: func(obj interface{}) {
			doDelete(obj)
		},
	})
	if err != nil {
		return nil, nil, nil, err
	}

	cancel := func() {
		_ = inf.RemoveEventHandler(handle)
	}
	return nil, ch, cancel, nil
}

func (s *ResultStoreSecrets) GetValidateResult(options GetValidateResultOptions) (*result.ValidateResult, error) {
	var l corev1.SecretList
	err := s.client.List(s.ctx, &l, client.MatchingLabels{
		"kluctl.io/validate-result-id": options.Id,
	})
	if err != nil {
		return nil, err
	}
	if len(l.Items) == 0 {
		return nil, nil
	}

	secret := l.Items[0]

	j, ok := secret.Data["result"]
	if !ok {
		return nil, fmt.Errorf("result field not present for %s", options.Id)
	}
	j, err = utils.UncompressGzip(j)
	if err != nil {
		return nil, err
	}

	var vr result.ValidateResult
	err = yaml.ReadYamlBytes(j, &vr)
	if err != nil {
		return nil, err
	}

	return &vr, nil
}
