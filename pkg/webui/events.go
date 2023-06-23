package webui

import (
	"container/list"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"net/http"
	"strings"
	"sync"
	"time"
)

var expireValidations = time.Minute * 10
var expireDeletions = time.Minute * 10

type eventsHandler struct {
	server *CommandResultsServer

	nextSeq   int64
	events    *list.List
	eventsMap map[string]*list.Element
	mutex     sync.Mutex
}

type eventEntry struct {
	id            string
	expire        *time.Time
	seq           int64
	projectTarget ProjectTargetKey
	payload       string
}

func newEventsHandler(server *CommandResultsServer) *eventsHandler {
	h := &eventsHandler{
		server:    server,
		events:    list.New(),
		eventsMap: map[string]*list.Element{},
	}

	return h
}

func (h *eventsHandler) updateEvent(id string, ptKey ProjectTargetKey, payload string, expireIn *time.Duration) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	seq := h.nextSeq
	h.nextSeq++

	var expire *time.Time
	if expireIn != nil {
		t := time.Now().Add(*expireIn)
		expire = &t
	}

	e, ok := h.eventsMap[id]
	if !ok {
		e = h.events.PushBack(&eventEntry{
			expire:        expire,
			seq:           seq,
			id:            id,
			projectTarget: ptKey,
			payload:       payload,
		})
		h.eventsMap[id] = e
	} else {
		e2 := e.Value.(*eventEntry)
		e2.expire = expire
		e2.seq = seq
		e2.payload = payload

		h.events.MoveToBack(e)
	}
}

func (h *eventsHandler) removeEvent(id string) {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	e, ok := h.eventsMap[id]
	if !ok {
		return
	}

	h.events.Remove(e)
	delete(h.eventsMap, id)
}

func (h *eventsHandler) cleanupEvents() {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	now := time.Now()

	var toDelete []*list.Element
	for e := h.events.Front(); e != nil; e = e.Next() {
		e2 := e.Value.(*eventEntry)
		if e2.expire != nil && now.After(*e2.expire) {
			toDelete = append(toDelete, e)
		}
	}

	for _, e := range toDelete {
		e2 := e.Value.(*eventEntry)
		delete(h.eventsMap, e2.id)
		h.events.Remove(e)
	}
}

func (h *eventsHandler) startEventsWatcher() error {
	h.mutex.Lock()
	defer h.mutex.Unlock()

	initial, ch, _, err := h.server.store.WatchCommandResultSummaries(results.ListCommandResultSummariesOptions{})
	if err != nil {
		return err
	}

	initialValidations, validationsCh, _ := h.server.vam.watch()

	ctx := h.server.ctx

	buildMsg := func(event results.WatchCommandResultSummaryEvent) string {
		if event.Delete {
			x := yaml.WriteJsonStringMust(map[string]any{
				"type": "delete_summary",
				"id":   event.Summary.Id,
			})
			return x
		} else {
			x := yaml.WriteJsonStringMust(map[string]any{
				"type":    "update_summary",
				"summary": event.Summary,
			})
			return x
		}
	}
	buildValidationMsg := func(event validationEvent) string {
		x := yaml.WriteJsonStringMust(map[string]any{
			"type":   "validate_result",
			"key":    event.key,
			"result": event.r,
		})
		return x
	}

	for _, x := range initial {
		h.updateEvent(x.Id, ProjectTargetKey{Project: x.ProjectKey, Target: x.TargetKey}, buildMsg(results.WatchCommandResultSummaryEvent{Summary: x}), nil)
	}
	for _, x := range initialValidations {
		h.updateEvent(uuid.NewString(), x.key, buildValidationMsg(x), &expireValidations)
	}

	go func() {
		cleanupTimer := time.After(5 * time.Second)
		for {
			select {
			case event, ok := <-ch:
				if !ok {
					status.Error(h.server.ctx, "results channel closed unexpectedly")
					return
				}
				var expireIn *time.Duration
				if event.Delete {
					expireIn = &expireDeletions
				}
				h.updateEvent(event.Summary.Id, ProjectTargetKey{Project: event.Summary.ProjectKey, Target: event.Summary.TargetKey}, buildMsg(event), expireIn)
			case event, ok := <-validationsCh:
				if !ok {
					status.Error(h.server.ctx, "validations channel closed unexpectedly")
					return
				}
				h.updateEvent(uuid.NewString(), event.key, buildValidationMsg(event), &expireValidations)
			case <-cleanupTimer:
				h.cleanupEvents()
				cleanupTimer = time.After(5 * time.Second)
			case <-ctx.Done():
				return
			}
		}
	}()

	return nil
}

func (h *eventsHandler) handler(c *gin.Context) {
	args := struct {
		FilterProject string `form:"filterProject"`
		FilterSubDir  string `form:"filterSubDir"`
		Seq           int64  `form:"seq"`
	}{}
	err := c.BindQuery(&args)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	repoKey, err := types.ParseGitRepoKey(args.FilterProject)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var filter *result.ProjectKey
	if args.FilterProject != "" {
		filter = &result.ProjectKey{
			GitRepoKey: repoKey,
			SubDir:     args.FilterSubDir,
		}
	}

	getNewEvents := func() ([]string, int64) {
		h.mutex.Lock()
		defer h.mutex.Unlock()
		var events []string
		nextSeq := args.Seq
		for e := h.events.Front(); e != nil; e = e.Next() {
			e2 := e.Value.(*eventEntry)
			if e2.seq < args.Seq {
				continue
			}
			nextSeq = e2.seq + 1
			if !results.FilterProject(e2.projectTarget.Project, filter) {
				continue
			}
			events = append(events, e2.payload)
		}

		return events, nextSeq
	}

	events, nextSeq := getNewEvents()
	timeout := time.After(30 * time.Second)
outer:
	for len(events) == 0 {
		select {
		case <-h.server.ctx.Done():
			_ = c.AbortWithError(http.StatusServiceUnavailable, fmt.Errorf("context cancelled"))
			return
		case <-timeout:
			break outer
		case <-time.After(100 * time.Millisecond):
		}

		events, nextSeq = getNewEvents()
	}

	j := fmt.Sprintf(`{"nextSeq": %d, "events": [%s]}`, nextSeq, strings.Join(events, ","))
	c.Writer.Header().Set("Content-Type", "application/json")
	c.Status(http.StatusOK)
	_, _ = c.Writer.WriteString(j)
}
