package webui

import (
	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"strings"
)

func acceptWebsocket(ctx *gin.Context) (*websocket.Conn, error) {
	acceptOptions := &websocket.AcceptOptions{
		InsecureSkipVerify: true,
	}

	userAgentLower := strings.ToLower(ctx.GetHeader("User-Agent"))
	isSafari := strings.Contains(userAgentLower, "safari") && !strings.Contains(userAgentLower, "chrome") && !strings.Contains(userAgentLower, "android")

	if isSafari {
		acceptOptions.CompressionMode = websocket.CompressionDisabled
	}

	conn, err := websocket.Accept(ctx.Writer, ctx.Request, acceptOptions)
	if err != nil {
		return nil, err
	}

	return conn, err
}
