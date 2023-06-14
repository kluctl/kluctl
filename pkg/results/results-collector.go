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

	resultSummaries map[string]summaryEntry
	watches         []*watchEntry

	mutex sync.Mutex
}

type summaryEntry struct {
	store   ResultStore
	summary *result.CommandResultSummary
}

type watchEntry struct {
	options ListCommandResultSummariesOptions
	ch      chan WatchCommandResultSummaryEvent

	initialQueue []*result.CommandResultSummary
}

func NewResultsCollector(ctx context.Context, stores []ResultStore) *ResultsCollector {
	ret := &ResultsCollector{
		ctx:             ctx,
		stores:          stores,
		resultSummaries: map[string]summaryEntry{},
	}

	return ret
}

func (rc *ResultsCollector) Start() {
	rc.startWatchResults()
}

func (rc *ResultsCollector) WaitForResults(idleTimeout time.Duration, totalTimeout time.Duration) error {
	_, ch, cancel, err := rc.WatchCommandResultSummaries(ListCommandResultSummariesOptions{})
	if err != nil {
		return err
	}
	defer cancel()

	totalCh := time.After(totalTimeout)
	for {
		idleCh := time.After(idleTimeout)
		select {
		case <-ch:
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
		go rc.runWatchResults(store)
	}
}

func (rc *ResultsCollector) runWatchResults(store ResultStore) {
	for {
		l, ch, _, err := store.WatchCommandResultSummaries(ListCommandResultSummariesOptions{})
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		for _, rs := range l {
			rc.handleUpdate(store, WatchCommandResultSummaryEvent{
				Summary: rs,
			})
		}

		go func() {
			for {
				event, ok := <-ch
				if !ok {
					break
				}
				rc.handleUpdate(store, event)
			}
		}()

		break
	}
}

func (rc *ResultsCollector) handleUpdate(store ResultStore, event WatchCommandResultSummaryEvent) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	if event.Delete {
		_, ok := rc.resultSummaries[event.Summary.Id]
		if ok {
			delete(rc.resultSummaries, event.Summary.Id)
		}
	} else {
		rc.resultSummaries[event.Summary.Id] = summaryEntry{
			store:   store,
			summary: event.Summary,
		}
	}

	for _, w := range rc.watches {
		if FilterSummary(event.Summary, w.options.ProjectFilter) {
			w.ch <- event
		}
	}
}

func (rc *ResultsCollector) WriteCommandResult(cr *result.CommandResult) error {
	return fmt.Errorf("WriteCommandResult is not supported in ResultsCollector")
}

func (rc *ResultsCollector) ListCommandResultSummaries(options ListCommandResultSummariesOptions) ([]result.CommandResultSummary, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	summaries := make([]result.CommandResultSummary, 0, len(rc.resultSummaries))
	for _, x := range rc.resultSummaries {
		if !FilterSummary(x.summary, options.ProjectFilter) {
			continue
		}
		summaries = append(summaries, *x.summary)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return lessSummary(&summaries[i], &summaries[j])
	})
	return summaries, nil
}

func (rc *ResultsCollector) WatchCommandResultSummaries(options ListCommandResultSummariesOptions) ([]*result.CommandResultSummary, <-chan WatchCommandResultSummaryEvent, context.CancelFunc, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	w := &watchEntry{
		options: options,
		ch:      make(chan WatchCommandResultSummaryEvent),
	}

	rc.watches = append(rc.watches, w)

	var initial []*result.CommandResultSummary

	for _, se := range rc.resultSummaries {
		if FilterSummary(se.summary, options.ProjectFilter) {
			initial = append(initial, se.summary)
		}
	}

	cancel := func() {
		rc.mutex.Lock()
		for i, w2 := range rc.watches {
			if w2 == w {
				rc.watches = append(rc.watches[:i], rc.watches[i+1:]...)
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
	_, ok := rc.resultSummaries[id]
	return ok, nil
}

func (rc *ResultsCollector) GetCommandResultSummary(id string) (*result.CommandResultSummary, error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	rs, ok := rc.resultSummaries[id]
	if !ok {
		return nil, nil
	}
	return rs.summary, nil
}

func (rc *ResultsCollector) GetCommandResult(options GetCommandResultOptions) (*result.CommandResult, error) {
	rc.mutex.Lock()
	se, ok := rc.resultSummaries[options.Id]
	rc.mutex.Unlock()
	if !ok {
		return nil, nil
	}
	return se.store.GetCommandResult(options)
}
