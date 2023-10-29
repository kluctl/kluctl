package webui

import (
	"bytes"
	"container/list"
	"context"
	"github.com/gin-gonic/gin"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"net/http"
	"nhooyr.io/websocket"
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
	projectTarget *ProjectTargetKey
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

func (h *eventsHandler) updateEvent(id string, ptKey *ProjectTargetKey, payload string, expireIn *time.Duration) {
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

	commandResultsCh, _, err := h.server.store.WatchCommandResultSummaries(results.ListResultSummariesOptions{})
	if err != nil {
		return err
	}
	validateResultsCh, _, err := h.server.store.WatchValidateResultSummaries(results.ListResultSummariesOptions{})
	if err != nil {
		return err
	}
	kluctlDeploymentsCh, _, err := h.server.store.WatchKluctlDeployments()
	if err != nil {
		return err
	}

	ctx := h.server.ctx

	buildCommandResultMsg := func(event results.WatchCommandResultSummaryEvent) string {
		if event.Delete {
			x := yaml.WriteJsonStringMust(map[string]any{
				"type": "delete_command_result_summary",
				"id":   event.Summary.Id,
			})
			return x
		} else {
			x := yaml.WriteJsonStringMust(map[string]any{
				"type":    "update_command_result_summary",
				"summary": event.Summary,
			})
			return x
		}
	}
	buildValidateResultMsg := func(event results.WatchValidateResultSummaryEvent) string {
		if event.Delete {
			x := yaml.WriteJsonStringMust(map[string]any{
				"type": "delete_validate_result_summary",
				"id":   event.Summary.Id,
			})
			return x
		} else {
			x := yaml.WriteJsonStringMust(map[string]any{
				"type":    "update_validate_result_summary",
				"summary": event.Summary,
			})
			return x
		}
	}
	buildKluctlDeploymentMsg := func(event results.WatchKluctlDeploymentEvent) string {
		if event.Delete {
			x := yaml.WriteJsonStringMust(map[string]any{
				"type":      "delete_kluctl_deployment",
				"id":        string(event.Deployment.UID),
				"clusterId": event.ClusterId,
			})
			return x
		} else {
			x := yaml.WriteJsonStringMust(map[string]any{
				"type":       "update_kluctl_deployment",
				"deployment": event.Deployment,
				"clusterId":  event.ClusterId,
			})
			return x
		}
	}

	go func() {
		cleanupTimer := time.After(5 * time.Second)
		for {
			select {
			case event, ok := <-commandResultsCh:
				if !ok {
					status.Error(h.server.ctx, "results channel closed unexpectedly")
					return
				}
				var expireIn *time.Duration
				if event.Delete {
					expireIn = &expireDeletions
				}
				h.updateEvent("cr-"+event.Summary.Id, &ProjectTargetKey{Project: event.Summary.ProjectKey, Target: event.Summary.TargetKey}, buildCommandResultMsg(event), expireIn)
			case event, ok := <-validateResultsCh:
				if !ok {
					status.Error(h.server.ctx, "results channel closed unexpectedly")
					return
				}
				var expireIn *time.Duration
				if event.Delete {
					expireIn = &expireDeletions
				}
				h.updateEvent("vr-"+event.Summary.Id, &ProjectTargetKey{Project: event.Summary.ProjectKey, Target: event.Summary.TargetKey}, buildValidateResultMsg(event), expireIn)
			case event, ok := <-kluctlDeploymentsCh:
				if !ok {
					status.Error(h.server.ctx, "results channel closed unexpectedly")
					return
				}
				var expireIn *time.Duration
				if event.Delete {
					expireIn = &expireDeletions
				}
				h.updateEvent("kd-"+string(event.Deployment.UID), nil, buildKluctlDeploymentMsg(event), expireIn)
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
	}{}
	err := c.BindQuery(&args)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	repoKey, err := types.ParseRepoKey(args.FilterProject, "git")
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var filter *result.ProjectKey
	if args.FilterProject != "" {
		filter = &result.ProjectKey{
			RepoKey: repoKey,
			SubDir:  args.FilterSubDir,
		}
	}

	conn, err := acceptWebsocket(c)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusInternalError, "the sky is falling")

	err = h.wsHandle(conn, filter)
	if err != nil {
		cs := websocket.CloseStatus(err)
		if cs == websocket.StatusNormalClosure || cs == websocket.StatusGoingAway {
			return
		}
		_ = c.AbortWithError(http.StatusInternalServerError, err)
	}
}

func (h *eventsHandler) wsHandle(c *websocket.Conn, filter *result.ProjectKey) error {
	ctx := c.CloseRead(h.server.ctx)

	getNewEvents := func(seq int64) ([]string, int64) {
		h.mutex.Lock()
		defer h.mutex.Unlock()
		var events []string
		nextSeq := seq
		for e := h.events.Front(); e != nil; e = e.Next() {
			e2 := e.Value.(*eventEntry)
			if e2.seq < seq {
				continue
			}
			nextSeq = e2.seq + 1
			if e2.projectTarget != nil && !results.FilterProject(e2.projectTarget.Project, filter) {
				continue
			}
			events = append(events, e2.payload)
		}

		return events, nextSeq
	}

	var seq int64
	for {
		events, nextSeq := getNewEvents(seq)
		if len(events) != 0 {
			buf := bytes.NewBuffer(nil)
			buf.Write([]byte("["))
			for i, e := range events {
				if i != 0 {
					buf.Write([]byte(","))
				}
				buf.Write([]byte(e))
			}
			buf.Write([]byte("]"))

			err := h.wsSendMessage(ctx, c, time.Second*5, buf.String())
			if err != nil {
				return err
			}
		}

		seq = nextSeq

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1000 * time.Millisecond):
		}
	}
}

func (s *eventsHandler) wsSendMessage(ctx context.Context, c *websocket.Conn, timeout time.Duration, msg string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := c.Write(ctx, websocket.MessageText, []byte(msg))
	if err != nil {
		return err
	}
	return err
}
