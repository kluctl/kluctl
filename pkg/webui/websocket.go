package webui

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"net/http"
	"nhooyr.io/websocket"
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

	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	})
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

	sendMessage := func(msg string) error {
		ctx, cancel := context.WithTimeout(s.ctx, 500*time.Millisecond)
		defer cancel()
		w, err := conn.Writer(ctx, websocket.MessageText)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte(msg))
		if err != nil {
			_ = w.Close()
			return err
		}
		err = w.Close()
		if err != nil {
			ctx = nil
		}
		return err
	}

	cancel, err := s.collector.WatchCommandResultSummaries(results.ListCommandResultSummariesOptions{ProjectFilter: filter},
		func(summary *result.CommandResultSummary) {
			x := yaml.WriteJsonStringMust(map[string]any{
				"type":    "update_summary",
				"summary": summary,
			})
			_ = sendMessage(x)
		}, func(id string) {
			x := yaml.WriteJsonStringMust(map[string]any{
				"type": "delete_summary",
				"id":   id,
			})
			_ = sendMessage(x)
		},
	)
	if err != nil {
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
	defer cancel()

	h := s.vam.addHandler(func(key ProjectTargetKey, r *result.ValidateResult) {
		x := yaml.WriteJsonStringMust(map[string]any{
			"type":   "validate_result",
			"key":    key,
			"result": r,
		})
		_ = sendMessage(x)
	})
	defer h()

	_, _, err = conn.Read(s.ctx)
	if err != nil {
		cs := websocket.CloseStatus(err)
		if cs == websocket.StatusNormalClosure || cs == websocket.StatusGoingAway {
			return
		}
		_ = c.AbortWithError(http.StatusInternalServerError, err)
		return
	}
}
