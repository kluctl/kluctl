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
	handlers        []*handlerEntry

	mutex sync.Mutex
}

type summaryEntry struct {
	store   ResultStore
	summary *result.CommandResultSummary
}

type handlerEntry struct {
	options ListCommandResultSummariesOptions
	update  func(result *result.CommandResultSummary)
	delete  func(id string)
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

func (rc *ResultsCollector) startWatchResults() {
	for _, store := range rc.stores {
		go rc.runWatchResults(store)
	}
}

func (rc *ResultsCollector) runWatchResults(store ResultStore) {
	for {
		_, err := store.WatchCommandResultSummaries(ListCommandResultSummariesOptions{}, func(summary *result.CommandResultSummary) {
			rc.handleUpdate(store, summary)
		}, func(id string) {
			rc.handleDelete(store, id)
		})
		if err == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}
}

func (rc *ResultsCollector) handleUpdate(store ResultStore, summary *result.CommandResultSummary) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	rc.resultSummaries[summary.Id] = summaryEntry{
		store:   store,
		summary: summary,
	}
	for _, h := range rc.handlers {
		if FilterSummary(summary, h.options.ProjectFilter) {
			h.update(summary)
		}
	}
}

func (rc *ResultsCollector) handleDelete(store ResultStore, id string) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()
	se, ok := rc.resultSummaries[id]
	if !ok {
		return
	}

	delete(rc.resultSummaries, id)
	for _, h := range rc.handlers {
		if FilterSummary(se.summary, h.options.ProjectFilter) {
			h.delete(id)
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

func (rc *ResultsCollector) WatchCommandResultSummaries(options ListCommandResultSummariesOptions, update func(summary *result.CommandResultSummary), delete func(id string)) (func(), error) {
	rc.mutex.Lock()
	defer rc.mutex.Unlock()

	h := &handlerEntry{
		options: options,
		update:  update,
		delete:  delete,
	}

	rc.handlers = append(rc.handlers, h)

	for _, se := range rc.resultSummaries {
		if FilterSummary(se.summary, h.options.ProjectFilter) {
			h.update(se.summary)
		}
	}

	return func() {
		rc.mutex.Lock()
		defer rc.mutex.Unlock()
		for i, h2 := range rc.handlers {
			if h2 == h {
				rc.handlers = append(rc.handlers[:i], rc.handlers[i+1:]...)
				break
			}
		}
	}, nil
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
