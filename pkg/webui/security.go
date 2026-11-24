package webui

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	gsessions "github.com/gorilla/sessions"
)

// crossOriginProtection rejects cross-origin requests with unsafe methods (POST, PUT, ...). Browsers identify these
// via Sec-Fetch-Site or, for older browsers, via an Origin that does not match the Host. Requests without either
// header (non-browser clients) are allowed.
func crossOriginProtection() gin.HandlerFunc {
	cop := http.NewCrossOriginProtection()
	return func(c *gin.Context) {
		if err := cop.Check(c.Request); err != nil {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	}
}

// securityHeaders sets a restrictive CSP and related headers. The UI bundle contains no inline scripts and only
// loads same-origin assets. Inline styles are needed by Radix (scroll locking injects a <style> element).
func securityHeaders() gin.HandlerFunc {
	return func(c *gin.Context) {
		host := c.Request.Host
		h := c.Writer.Header()
		h.Set("Content-Security-Policy", strings.Join([]string{
			"default-src 'self'",
			"script-src 'self'",
			"style-src 'self' 'unsafe-inline'",
			"img-src 'self' data:",
			"font-src 'self'",
			// Older Safari versions don't match ws(s) against 'self'.
			fmt.Sprintf("connect-src 'self' ws://%s wss://%s", host, host),
			"object-src 'none'",
			"base-uri 'self'",
			"form-action 'self'",
			"frame-ancestors 'none'",
		}, "; "))
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		c.Next()
	}
}

func newSessionStore(secret []byte, rootPath string) sessions.Store {
	store := cookie.NewStore(secret)
	store.Options(sessions.Options{
		Path:     rootPath,
		MaxAge:   86400 * 30,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	return secureCookieStore{store}
}

// secureCookieStore marks the session cookie Secure whenever the browser talks HTTPS to us (directly or through a
// TLS terminating proxy). Deciding this per request keeps plain-HTTP setups like port-forwarding working.
type secureCookieStore struct {
	cookie.Store
}

func (s secureCookieStore) Get(r *http.Request, name string) (*gsessions.Session, error) {
	// Each session carries its own copy of the store options, which are used when it is saved.
	session, err := s.Store.Get(r, name)
	if session != nil {
		session.Options.Secure = r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	}
	return session, err
}
