package webui

import (
	"context"
	"encoding/json"
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"github.com/kluctl/kluctl/v2/pkg/controllers/logs"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

func (s *CommandResultsServer) logsHandler(gctx *gin.Context) {
	args := struct {
		KluctlDeploymentParam
		ReconcileId string `form:"reconcileId"`
	}{}
	err := gctx.BindQuery(&args)
	if err != nil {
		_ = gctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if s.serverCoreV1Client == nil {
		_ = gctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	conn, err := acceptWebsocket(gctx)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusInternalError, "the sky is falling")

	ctx := conn.CloseRead(gctx)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logCh, err := logs.WatchControllerLogs(ctx, s.serverCoreV1Client, s.controllerNamespace, client.ObjectKey{Name: args.Name, Namespace: args.Namespace}, args.ReconcileId, 60*time.Second, true)
	if err != nil {
		_ = gctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	for l := range logCh {
		var b []byte
		b, err = json.Marshal(&l)
		if err != nil {
			return
		}
		err = conn.Write(ctx, websocket.MessageText, b)
		if err != nil {
			return
		}
	}
}
