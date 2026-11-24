package webui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/coder/websocket"
	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func newSecurityTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	auth := &authHandler{
		authConfig:     AuthConfig{AuthEnabled: true, StaticLoginEnabled: true},
		authSecret:     []byte("test-session-key"),
		viewerPassword: "test-viewer-password",
		oidcProvider:   &oidc.Provider{},
	}
	engine := gin.New()
	engine.Use(securityHeaders(), crossOriginProtection())
	group := engine.Group("/tools/kluctl")
	require.NoError(t, auth.setupRoutes(group, false))
	group.POST("/api/echo", func(c *gin.Context) {
		var params KluctlDeploymentParam
		if err := c.ShouldBindJSON(&params); err != nil {
			_ = c.AbortWithError(http.StatusBadRequest, err)
			return
		}
		c.JSON(http.StatusOK, params)
	})
	group.GET("/api/ws", func(c *gin.Context) {
		conn, err := acceptWebsocket(c)
		if err != nil {
			return
		}
		_ = conn.Close(websocket.StatusNormalClosure, "")
	})
	server := httptest.NewServer(engine)
	t.Cleanup(server.Close)
	return server
}

func doRequest(t *testing.T, method, url, contentType, body string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	require.NoError(t, err)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r, err := http.DefaultTransport.RoundTrip(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = r.Body.Close() })
	return r
}

func TestWebUICrossOriginPostsRejected(t *testing.T) {
	server := newSecurityTestServer(t)
	echo := server.URL + "/tools/kluctl/api/echo"
	body := `{"cluster":"c","name":"n","namespace":"ns"}`

	for name, headers := range map[string]map[string]string{
		"cross-site":      {"Sec-Fetch-Site": "cross-site"},
		"same-site":       {"Sec-Fetch-Site": "same-site"},
		"foreign origin":  {"Origin": "https://evil.example"},
		"spoofed referer": {"Origin": "https://evil.example", "Referer": server.URL + "/"},
	} {
		t.Run(name, func(t *testing.T) {
			r := doRequest(t, http.MethodPost, echo, "application/json", body, headers)
			require.Equal(t, http.StatusForbidden, r.StatusCode)
		})
	}

	r := doRequest(t, http.MethodPost, echo, "application/json", body, map[string]string{"Sec-Fetch-Site": "same-origin"})
	require.Equal(t, http.StatusOK, r.StatusCode)
	r = doRequest(t, http.MethodPost, echo, "application/json", body, map[string]string{"Origin": server.URL})
	require.Equal(t, http.StatusOK, r.StatusCode)

	// Form posts don't need a CORS preflight, so the API only accepts JSON.
	r = doRequest(t, http.MethodPost, echo, "application/x-www-form-urlencoded", "cluster=c&name=n&namespace=ns", nil)
	require.Equal(t, http.StatusBadRequest, r.StatusCode)
	r = doRequest(t, http.MethodPost, server.URL+"/tools/kluctl/auth/staticLogin", "application/x-www-form-urlencoded",
		"username=viewer&password=test-viewer-password", nil)
	require.Equal(t, http.StatusUnauthorized, r.StatusCode)
}

func TestWebUIWebsocketOrigin(t *testing.T) {
	server := newSecurityTestServer(t)
	wsUrl := "ws" + strings.TrimPrefix(server.URL, "http") + "/tools/kluctl/api/ws"

	dial := func(headers map[string]string) (*http.Response, error) {
		h := http.Header{}
		for k, v := range headers {
			h.Set(k, v)
		}
		conn, r, err := websocket.Dial(context.Background(), wsUrl, &websocket.DialOptions{HTTPHeader: h})
		if conn != nil {
			_ = conn.CloseNow()
		}
		return r, err
	}

	for name, headers := range map[string]map[string]string{
		"cross-site":     {"Sec-Fetch-Site": "cross-site", "Origin": "https://evil.example"},
		"same-site":      {"Sec-Fetch-Site": "same-site", "Origin": "https://sub.example"},
		"foreign origin": {"Origin": "https://evil.example"},
	} {
		t.Run(name, func(t *testing.T) {
			r, err := dial(headers)
			require.Error(t, err)
			require.Equal(t, http.StatusForbidden, r.StatusCode)
		})
	}

	for name, headers := range map[string]map[string]string{
		"same-origin":           {"Sec-Fetch-Site": "same-origin", "Origin": server.URL},
		"same-origin via proxy": {"Sec-Fetch-Site": "same-origin", "Origin": "https://kluctl.example"},
		"matching origin":       {"Origin": server.URL},
		"non-browser client":    nil,
	} {
		t.Run(name, func(t *testing.T) {
			r, err := dial(headers)
			require.NoError(t, err)
			require.Equal(t, http.StatusSwitchingProtocols, r.StatusCode)
		})
	}
}

func TestWebUISecurityHeadersAndCookie(t *testing.T) {
	server := newSecurityTestServer(t)
	login := func(headers map[string]string) *http.Cookie {
		r := doRequest(t, http.MethodPost, server.URL+"/tools/kluctl/auth/staticLogin", "application/json",
			`{"username":"viewer","password":"test-viewer-password"}`, headers)
		require.Equal(t, http.StatusOK, r.StatusCode)
		require.Contains(t, r.Header.Get("Content-Security-Policy"), "frame-ancestors 'none'")
		require.Contains(t, r.Header.Get("Content-Security-Policy"), "script-src 'self'")
		require.Equal(t, "DENY", r.Header.Get("X-Frame-Options"))
		require.Equal(t, "nosniff", r.Header.Get("X-Content-Type-Options"))
		cookies := r.Cookies()
		require.Len(t, cookies, 1)
		return cookies[0]
	}

	c := login(nil)
	require.Equal(t, "auth-session", c.Name)
	require.Equal(t, "/tools/kluctl/", c.Path)
	require.True(t, c.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, c.SameSite)
	require.False(t, c.Secure)

	c = login(map[string]string{"X-Forwarded-Proto": "https"})
	require.True(t, c.Secure)
}
