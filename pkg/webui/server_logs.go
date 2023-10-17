package webui

import (
	"context"
	"github.com/gin-gonic/gin"
	"github.com/kluctl/kluctl/v2/pkg/controllers/logs"
	"net/http"
	"nhooyr.io/websocket"
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

	controllerNamespace := "kluctl-system"

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	logCh, err := logs.WatchControllerLogs(ctx, s.serverCoreV1Client, controllerNamespace, client.ObjectKey{Name: args.Name, Namespace: args.Namespace}, args.ReconcileId, 60*time.Second)
	if err != nil {
		_ = gctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	for l := range logCh {
		err = conn.Write(ctx, websocket.MessageText, []byte(l))
		if err != nil {
			return
		}
	}
}
