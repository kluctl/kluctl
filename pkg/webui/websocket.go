package webui

import (
	"context"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"net/http"
	"nhooyr.io/websocket"
	"strings"
	"time"
)

func (s *CommandResultsServer) ws(c *gin.Context) {
	args := struct {
		FilterProject string `form:"filterProject"`
		FilterSubDir  string `form:"filterSubDir"`
	}{}
	err := c.BindQuery(&args)
	if err != nil {
		_ = c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	acceptOptions := &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	}

	userAgentLower := strings.ToLower(c.GetHeader("User-Agent"))
	isSafari := strings.Contains(userAgentLower, "safari") && !strings.Contains(userAgentLower, "chrome") && !strings.Contains(userAgentLower, "android")

	if isSafari {
		acceptOptions.CompressionMode = websocket.CompressionDisabled
	}

	conn, err := websocket.Accept(c.Writer, c.Request, acceptOptions)
	if err != nil {
		//s.logf("%v", err)
		return
	}
	defer conn.Close(websocket.StatusInternalError, "the sky is falling")

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

	err = s.wsHandle(conn, filter)
	if err != nil {
		cs := websocket.CloseStatus(err)
		if cs == websocket.StatusNormalClosure || cs == websocket.StatusGoingAway {
			return
		}
		_ = c.AbortWithError(http.StatusInternalServerError, err)
	}
}

func (s *CommandResultsServer) wsHandle(c *websocket.Conn, filter *result.ProjectKey) error {
	initial, ch, cancel, err := s.store.WatchCommandResultSummaries(results.ListCommandResultSummariesOptions{ProjectFilter: filter})
	if err != nil {
		return err
	}
	defer cancel()

	initialValidations, validationsCh, validationsCancel := s.vam.watch()
	defer validationsCancel()

	ctx := c.CloseRead(s.ctx)

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
		err := s.wsSendMessage(ctx, c, time.Second*5, buildMsg(results.WatchCommandResultSummaryEvent{Summary: x}))
		if err != nil {
			return err
		}
	}
	for _, x := range initialValidations {
		err := s.wsSendMessage(ctx, c, time.Second*5, buildValidationMsg(x))
		if err != nil {
			return err
		}
	}

	for {
		var msg string
		select {
		case event, ok := <-ch:
			if !ok {
				return fmt.Errorf("results channel closed unexpectadly")
			}
			msg = buildMsg(event)
		case event, ok := <-validationsCh:
			if !ok {
				return fmt.Errorf("validations channel closed unexpectadly")
			}
			msg = buildValidationMsg(event)
		case <-ctx.Done():
			return ctx.Err()
		}
		err := s.wsSendMessage(ctx, c, time.Second*5, msg)
		if err != nil {
			return err
		}
	}
}

func (s *CommandResultsServer) wsSendMessage(ctx context.Context, c *websocket.Conn, timeout time.Duration, msg string) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	err := c.Write(ctx, websocket.MessageText, []byte(msg))
	if err != nil {
		return err
	}
	return err
}
