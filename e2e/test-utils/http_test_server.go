package test_utils

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

type TestHttpServer struct {
	TLSEnabled bool
	Username   string
	Password   string

	Server *httptest.Server
}

func (s *TestHttpServer) Start(t *testing.T, h http.Handler) {
	authH := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if s.Username != "" || s.Password != "" {
			u, p, ok := request.BasicAuth()
			if !ok {
				writer.Header().Add("WWW-Authenticate", "Basic")
				http.Error(writer, "Auth header was incorrect", http.StatusUnauthorized)
				return
			}
			if u != s.Username || p != s.Password {
				http.Error(writer, "Auth header was incorrect", http.StatusUnauthorized)
				return
			}
		}
		h.ServeHTTP(writer, request)
	})

	if s.TLSEnabled {
		s.Server = httptest.NewUnstartedServer(authH)
		s.Server.StartTLS()
	} else {
		s.Server = httptest.NewServer(authH)
	}

	t.Cleanup(s.Server.Close)
}
