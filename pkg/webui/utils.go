package webui

import (
	"fmt"
	"net/http"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
	"strings"
)

func acceptWebsocket(ctx *gin.Context) (*websocket.Conn, error) {
	acceptOptions := &websocket.AcceptOptions{}

	// Browsers don't apply the same-origin policy to WebSockets, so we must reject cross-site handshakes ourselves.
	switch ctx.GetHeader("Sec-Fetch-Site") {
	case "same-origin":
		// The browser vouches for a same-origin request. This also holds behind proxies that rewrite the Host
		// header, where the Origin/Host comparison below would fail.
		acceptOptions.InsecureSkipVerify = true
	case "":
		// Older browsers and non-browser clients: websocket.Accept compares Origin (if present) with Host.
	default:
		ctx.AbortWithStatus(http.StatusForbidden)
		return nil, fmt.Errorf("rejected cross-origin websocket request")
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
