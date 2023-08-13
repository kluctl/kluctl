package webui

import (
	"bufio"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"nhooyr.io/websocket"
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

	conn, err := acceptWebsocket(gctx)
	if err != nil {
		return
	}
	defer conn.Close(websocket.StatusInternalError, "the sky is falling")

	ctx := conn.CloseRead(gctx)

	controllerNamespace := "kluctl-system"
	pods, err := s.serverCoreV1Client.Pods(controllerNamespace).List(s.ctx, metav1.ListOptions{
		LabelSelector: "control-plane=kluctl-controller",
	})
	if err != nil {
		_ = gctx.AbortWithError(http.StatusBadRequest, err)
		return
	}

	var streams []io.ReadCloser
	defer func() {
		for _, s := range streams {
			_ = s.Close()
		}
	}()

	lineCh := make(chan string)

	for _, pod := range pods.Items {
		since := int64(60 * 5)
		rc, err := s.serverCoreV1Client.Pods("kluctl-system").GetLogs(pod.Name, &corev1.PodLogOptions{
			Follow:       true,
			SinceSeconds: &since,
		}).Stream(s.ctx)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			_ = gctx.AbortWithError(http.StatusBadRequest, err)
			return
		}
		streams = append(streams, rc)
	}

	for _, s := range streams {
		s := s
		go func() {
			sc := bufio.NewScanner(s)
			for sc.Scan() {
				l := sc.Text()
				lineCh <- l
			}
		}()
	}

	for l := range lineCh {
		var j map[string]any
		err = json.Unmarshal([]byte(l), &j)
		if err != nil {
			continue
		}

		name := j["name"]
		namespace := j["namespace"]

		if args.Name != "" && name != args.Name {
			continue
		}
		if args.Namespace != "" && namespace != args.Namespace {
			continue
		}

		rid := j["reconcileID"]
		if args.ReconcileId != "" && rid != args.ReconcileId {
			continue
		}

		err = conn.Write(ctx, websocket.MessageText, []byte(l))
		if err != nil {
			return
		}
	}
}
