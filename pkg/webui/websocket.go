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

	// don't try anything else before we get the auth message
	user, err := s.wsHandleAuth(conn)
	if err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
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

	err = s.wsHandle(conn, user, filter)
	if err != nil {
		cs := websocket.CloseStatus(err)
		if cs == websocket.StatusNormalClosure || cs == websocket.StatusGoingAway {
			return
		}
		_ = c.AbortWithError(http.StatusInternalServerError, err)
	}
}

func (s *CommandResultsServer) wsHandleAuth(c *websocket.Conn) (*User, error) {
	if s.auth == nil {
		return nil, nil
	}

	t, msg, err := c.Read(s.ctx)
	if err != nil {
		return nil, err
	}
	if t != websocket.MessageText {
		return nil, fmt.Errorf("unexpected message type")
	}

	type authMsg struct {
		Type  string `json:"type"`
		Token string `json:"token"`
	}
	type authResult struct {
		Type    string `json:"type"`
		Success bool   `json:"success"`
	}
	var am authMsg
	err = yaml.ReadYamlString(string(msg), &am)
	if err != nil {
		return nil, err
	}
	if am.Type != "auth" {
		return nil, fmt.Errorf("unexpected message type")
	}

	user := s.auth.getUserFromToken(am.Token)
	if user == nil {
		msg, _ := yaml.WriteJsonString(authResult{
			Type:    "auth_result",
			Success: false,
		})
		_ = c.Write(s.ctx, websocket.MessageText, []byte(msg))
		return nil, fmt.Errorf("invalid token")
	} else {
		msg, _ := yaml.WriteJsonString(authResult{
			Type:    "auth_result",
			Success: true,
		})
		err = c.Write(s.ctx, websocket.MessageText, []byte(msg))
		if err != nil {
			return nil, err
		}
		return user, nil
	}
}

func (s *CommandResultsServer) wsHandle(c *websocket.Conn, user *User, filter *result.ProjectKey) error {
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
