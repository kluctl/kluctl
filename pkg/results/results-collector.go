package results

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
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

type commandResultWatchEntry struct {
	options ListResultSummariesOptions
	ch      chan WatchCommandResultSummaryEvent

	initialQueue []*result.CommandResultSummary
}

type validateResultWatchEntry struct {
	options ListResultSummariesOptions
	ch      chan WatchValidateResultSummaryEvent

	initialQueue []*result.ValidateResultSummary
}

func NewResultsCollector(ctx context.Context, stores []ResultStore) *ResultsCollector {
	ret := &ResultsCollector{
		ctx:                     ctx,
		stores:                  stores,
		commandResultSummaries:  map[string]commandSummaryEntry{},
		validateResultSummaries: map[string]validateSummaryEntry{},
	}

	return ret
}

func (rc *ResultsCollector) Start() {
	rc.startWatchResults()
}

func (rc *ResultsCollector) WaitForResults(idleTimeout time.Duration, totalTimeout time.Duration) error {
	_, ch1, cancel1, err := rc.WatchCommandResultSummaries(ListResultSummariesOptions{})
	if err != nil {
		return err
	}
	defer cancel1()

	_, ch2, cancel2, err := rc.WatchValidateResultSummaries(ListResultSummariesOptions{})
	if err != nil {
		return err
	}
	defer cancel2()

	totalCh := time.After(totalTimeout)
	for {
		idleCh := time.After(idleTimeout)
		select {
		case <-ch1:
			continue
		case <-ch2:
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
	}
}

func (rc *ResultsCollector) runWatchCommandResults(store ResultStore) {
	for {
		l, ch, _, err := store.WatchCommandResultSummaries(ListResultSummariesOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for _, rs := range l {
			rc.handleCommandResultUpdate(store, WatchCommandResultSummaryEvent{
				Summary: rs,
			})
		}

		go func() {
			for {
				event, ok := <-ch
				if !ok {
					break
				}
				rc.handleCommandResultUpdate(store, event)
			}
		}()

		break
	}
}

func (rc *ResultsCollector) runWatchValidateResults(store ResultStore) {
	for {
		l, ch, _, err := store.WatchValidateResultSummaries(ListResultSummariesOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for _, rs := range l {
			rc.handleValidateResultUpdate(store, WatchValidateResultSummaryEvent{
				Summary: rs,
			})
		}

		go func() {
			for {
				event, ok := <-ch
				if !ok {
					break
				}
				rc.handleValidateResultUpdate(store, event)
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

func (rc *ResultsCollector) WatchCommandResultSummaries(options ListResultSummariesOptions) ([]*result.CommandResultSummary, <-chan WatchCommandResultSummaryEvent, context.CancelFunc, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	w := &commandResultWatchEntry{
		options: options,
		ch:      make(chan WatchCommandResultSummaryEvent),
	}

	rc.commandResultWatches = append(rc.commandResultWatches, w)

	var initial []*result.CommandResultSummary

	for _, se := range rc.commandResultSummaries {
		if FilterProject(se.summary.ProjectKey, options.ProjectFilter) {
			initial = append(initial, se.summary)
		}
	}

	cancel := func() {
		rc.mutex.Lock()
		for i, w2 := range rc.commandResultWatches {
			if w2 == w {
				rc.commandResultWatches = append(rc.commandResultWatches[:i], rc.commandResultWatches[i+1:]...)
				break
			}
		}
		rc.mutex.Unlock()
		close(w.ch)
	}

	return initial, w.ch, cancel, nil
}

func (rc *ResultsCollector) HasCommandResult(id string) (bool, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	_, ok := rc.commandResultSummaries[id]
	return ok, nil
}

func (rc *ResultsCollector) GetCommandResultSummary(id string) (*result.CommandResultSummary, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	rs, ok := rc.commandResultSummaries[id]
	if !ok {
		return nil, nil
	}
	return rs.summary, nil
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

func (rc *ResultsCollector) WatchValidateResultSummaries(options ListResultSummariesOptions) ([]*result.ValidateResultSummary, <-chan WatchValidateResultSummaryEvent, context.CancelFunc, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	w := &validateResultWatchEntry{
		options: options,
		ch:      make(chan WatchValidateResultSummaryEvent),
	}

	rc.validateResultWatches = append(rc.validateResultWatches, w)

	var initial []*result.ValidateResultSummary

	for _, se := range rc.validateResultSummaries {
		if FilterProject(se.summary.ProjectKey, options.ProjectFilter) {
			initial = append(initial, se.summary)
		}
	}

	cancel := func() {
		rc.mutex.Lock()
		for i, w2 := range rc.validateResultWatches {
			if w2 == w {
				rc.validateResultWatches = append(rc.validateResultWatches[:i], rc.validateResultWatches[i+1:]...)
				break
			}
		}
		rc.mutex.Unlock()
		close(w.ch)
	}

	return initial, w.ch, cancel, nil
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
