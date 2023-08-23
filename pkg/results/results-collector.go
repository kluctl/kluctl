package results

import (
	"context"
	"fmt"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"k8s.io/apimachinery/pkg/types"
	"sort"
	"sync"
	"time"
)

type ResultsCollector struct {
	ctx context.Context

	stores []ResultStore

	commandResultSummaries map[string]commandSummaryEntry
	commandResultWatches   []*commandResultWatchEntry

	validateResultSummaries map[string]validateSummaryEntry
	validateResultWatches   []*validateResultWatchEntry

	kluctlDeployments       map[types.UID]kluctlDeploymentEntry
	kluctlDeploymentWatches []*kluctlDeploymentWatchEntry

	mutex sync.Mutex
}

type commandSummaryEntry struct {
	store   ResultStore
	summary *result.CommandResultSummary
}

type validateSummaryEntry struct {
	store   ResultStore
	summary *result.ValidateResultSummary
}

type kluctlDeploymentEntry struct {
	store ResultStore
	event WatchKluctlDeploymentEvent
}

type commandResultWatchEntry struct {
	options ListResultSummariesOptions
	ch      chan WatchCommandResultSummaryEvent
}

type validateResultWatchEntry struct {
	options ListResultSummariesOptions
	ch      chan WatchValidateResultSummaryEvent
}

type kluctlDeploymentWatchEntry struct {
	ch chan WatchKluctlDeploymentEvent
}

func NewResultsCollector(ctx context.Context, stores []ResultStore) *ResultsCollector {
	ret := &ResultsCollector{
		ctx:                     ctx,
		stores:                  stores,
		commandResultSummaries:  map[string]commandSummaryEntry{},
		validateResultSummaries: map[string]validateSummaryEntry{},
		kluctlDeployments:       map[types.UID]kluctlDeploymentEntry{},
	}

	return ret
}

func (rc *ResultsCollector) Start() {
	rc.startWatchResults()
}

func (rc *ResultsCollector) WaitForResults(idleTimeout time.Duration, totalTimeout time.Duration) error {
	ch1, cancel1, err := rc.WatchCommandResultSummaries(ListResultSummariesOptions{})
	if err != nil {
		return err
	}
	defer cancel1()

	ch2, cancel2, err := rc.WatchValidateResultSummaries(ListResultSummariesOptions{})
	if err != nil {
		return err
	}
	defer cancel2()

	ch3, cancel3, err := rc.WatchValidateResultSummaries(ListResultSummariesOptions{})
	if err != nil {
		return err
	}
	defer cancel3()

	totalCh := time.After(totalTimeout)
	for {
		idleCh := time.After(idleTimeout)
		select {
		case <-ch1:
			continue
		case <-ch2:
			continue
		case <-ch3:
			continue
		case <-idleCh:
			return nil
		case <-totalCh:
			return fmt.Errorf("result collector kept receiving results even after the total timeout")
		}
	}
}

func (rc *ResultsCollector) startWatchResults() {
	for _, store := range rc.stores {
		go rc.runWatchCommandResults(store)
		go rc.runWatchValidateResults(store)
		go rc.runWatchKluctlDeployments(store)
	}
}

func (rc *ResultsCollector) runWatchCommandResults(store ResultStore) {
	for {
		ch, _, err := store.WatchCommandResultSummaries(ListResultSummariesOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		go func() {
			for event := range ch {
				rc.handleCommandResultUpdate(store, event)
			}
		}()

		break
	}
}

func (rc *ResultsCollector) runWatchValidateResults(store ResultStore) {
	for {
		ch, _, err := store.WatchValidateResultSummaries(ListResultSummariesOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		go func() {
			for event := range ch {
				rc.handleValidateResultUpdate(store, event)
			}
		}()

		break
	}
}

func (rc *ResultsCollector) runWatchKluctlDeployments(store ResultStore) {
	for {
		ch, _, err := store.WatchKluctlDeployments()
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		go func() {
			for event := range ch {
				rc.handleKluctlDeploymentUpdate(store, event)
			}
		}()

		break
	}
}

func (rc *ResultsCollector) handleCommandResultUpdate(store ResultStore, event WatchCommandResultSummaryEvent) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	if event.Delete {
		_, ok := rc.commandResultSummaries[event.Summary.Id]
		if ok {
			delete(rc.commandResultSummaries, event.Summary.Id)
		}
	} else {
		rc.commandResultSummaries[event.Summary.Id] = commandSummaryEntry{
			store:   store,
			summary: event.Summary,
		}
	}

	for _, w := range rc.commandResultWatches {
		if FilterProject(event.Summary.ProjectKey, w.options.ProjectFilter) {
			w.ch <- event
		}
	}
}

func (rc *ResultsCollector) handleValidateResultUpdate(store ResultStore, event WatchValidateResultSummaryEvent) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	if event.Delete {
		_, ok := rc.validateResultSummaries[event.Summary.Id]
		if ok {
			delete(rc.validateResultSummaries, event.Summary.Id)
		}
	} else {
		rc.validateResultSummaries[event.Summary.Id] = validateSummaryEntry{
			store:   store,
			summary: event.Summary,
		}
	}

	for _, w := range rc.validateResultWatches {
		if FilterProject(event.Summary.ProjectKey, w.options.ProjectFilter) {
			w.ch <- event
		}
	}
}

func (rc *ResultsCollector) handleKluctlDeploymentUpdate(store ResultStore, event WatchKluctlDeploymentEvent) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	if event.Delete {
		_, ok := rc.kluctlDeployments[event.Deployment.UID]
		if ok {
			delete(rc.kluctlDeployments, event.Deployment.UID)
		}
	} else {
		rc.kluctlDeployments[event.Deployment.UID] = kluctlDeploymentEntry{
			store: store,
			event: event,
		}
	}

	for _, w := range rc.kluctlDeploymentWatches {
		w.ch <- event
	}
}

func (rc *ResultsCollector) WriteCommandResult(cr *result.CommandResult) error {
	return fmt.Errorf("WriteCommandResult is not supported in ResultsCollector")
}

func (rc *ResultsCollector) WriteValidateResult(vr *result.ValidateResult) error {
	return fmt.Errorf("WriteValidateResult is not supported in ResultsCollector")
}

func (rc *ResultsCollector) ListCommandResultSummaries(options ListResultSummariesOptions) ([]result.CommandResultSummary, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	summaries := make([]result.CommandResultSummary, 0, len(rc.commandResultSummaries))
	for _, x := range rc.commandResultSummaries {
		if !FilterProject(x.summary.ProjectKey, options.ProjectFilter) {
			continue
		}
		summaries = append(summaries, *x.summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return lessCommandSummary(&summaries[i], &summaries[j])
	})
	return summaries, nil
}

func (rc *ResultsCollector) WatchCommandResultSummaries(options ListResultSummariesOptions) (<-chan WatchCommandResultSummaryEvent, context.CancelFunc, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	isCancelled := false

	w := &commandResultWatchEntry{
		options: options,
		ch:      make(chan WatchCommandResultSummaryEvent),
	}

	go func() {
		rc.mutex.Lock()
		defer rc.mutex.Unlock()

		if isCancelled {
			return
		}

		for _, se := range rc.commandResultSummaries {
			if FilterProject(se.summary.ProjectKey, options.ProjectFilter) {
				w.ch <- WatchCommandResultSummaryEvent{
					Summary: se.summary,
				}
			}
		}

		rc.commandResultWatches = append(rc.commandResultWatches, w)
	}()

	cancel := func() {
		rc.mutex.Lock()
		isCancelled = true
		for i, w2 := range rc.commandResultWatches {
			if w2 == w {
				rc.commandResultWatches = append(rc.commandResultWatches[:i], rc.commandResultWatches[i+1:]...)
				break
			}
		}
		rc.mutex.Unlock()
		close(w.ch)
	}

	return w.ch, cancel, nil
}

func (rc *ResultsCollector) GetCommandResult(options GetCommandResultOptions) (*result.CommandResult, error) {
	rc.mutex.Lock()
	se, ok := rc.commandResultSummaries[options.Id]
	rc.mutex.Unlock()
	if !ok {
		return nil, nil
	}
	return se.store.GetCommandResult(options)
}

func (rc *ResultsCollector) ListValidateResultSummaries(options ListResultSummariesOptions) ([]result.ValidateResultSummary, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	summaries := make([]result.ValidateResultSummary, 0, len(rc.validateResultSummaries))
	for _, x := range rc.validateResultSummaries {
		if !FilterProject(x.summary.ProjectKey, options.ProjectFilter) {
			continue
		}
		summaries = append(summaries, *x.summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return lessValidateSummary(&summaries[i], &summaries[j])
	})
	return summaries, nil
}

func (rc *ResultsCollector) WatchValidateResultSummaries(options ListResultSummariesOptions) (<-chan WatchValidateResultSummaryEvent, context.CancelFunc, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	isCancelled := false

	w := &validateResultWatchEntry{
		options: options,
		ch:      make(chan WatchValidateResultSummaryEvent),
	}

	go func() {
		rc.mutex.Lock()
		defer rc.mutex.Unlock()

		if isCancelled {
			return
		}

		for _, se := range rc.validateResultSummaries {
			if FilterProject(se.summary.ProjectKey, options.ProjectFilter) {
				w.ch <- WatchValidateResultSummaryEvent{
					Summary: se.summary,
				}
			}
		}

		rc.validateResultWatches = append(rc.validateResultWatches, w)
	}()

	cancel := func() {
		rc.mutex.Lock()
		isCancelled = true
		for i, w2 := range rc.validateResultWatches {
			if w2 == w {
				rc.validateResultWatches = append(rc.validateResultWatches[:i], rc.validateResultWatches[i+1:]...)
				break
			}
		}
		rc.mutex.Unlock()
		close(w.ch)
	}

	return w.ch, cancel, nil
}

func (rc *ResultsCollector) GetValidateResult(options GetValidateResultOptions) (*result.ValidateResult, error) {
	rc.mutex.Lock()
	se, ok := rc.validateResultSummaries[options.Id]
	rc.mutex.Unlock()
	if !ok {
		return nil, nil
	}
	return se.store.GetValidateResult(options)
}

func (rc *ResultsCollector) ListKluctlDeployments() ([]WatchKluctlDeploymentEvent, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	ret := make([]WatchKluctlDeploymentEvent, 0, len(rc.kluctlDeployments))
	for _, x := range rc.kluctlDeployments {
		ret = append(ret, x.event)
	}
	return ret, nil
}

func (rc *ResultsCollector) WatchKluctlDeployments() (<-chan WatchKluctlDeploymentEvent, context.CancelFunc, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	isCancelled := false

	w := &kluctlDeploymentWatchEntry{
		ch: make(chan WatchKluctlDeploymentEvent),
	}

	go func() {
		rc.mutex.Lock()
		defer rc.mutex.Unlock()

		if isCancelled {
			return
		}

		for _, e := range rc.kluctlDeployments {
			w.ch <- e.event
		}

		rc.kluctlDeploymentWatches = append(rc.kluctlDeploymentWatches, w)
	}()

	cancel := func() {
		rc.mutex.Lock()
		isCancelled = true
		for i, w2 := range rc.kluctlDeploymentWatches {
			if w2 == w {
				rc.kluctlDeploymentWatches = append(rc.kluctlDeploymentWatches[:i], rc.kluctlDeploymentWatches[i+1:]...)
				break
			}
		}
		rc.mutex.Unlock()
		close(w.ch)
	}

	return w.ch, cancel, nil
}

func (rc *ResultsCollector) GetKluctlDeployment(clusterId string, name string, namespace string) (*kluctlv1.KluctlDeployment, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	for _, kd := range rc.kluctlDeployments {
		if kd.event.ClusterId == clusterId && kd.event.Deployment.Name == name && kd.event.Deployment.Namespace == namespace {
			return kd.event.Deployment, nil
		}
	}
	return nil, nil
}
