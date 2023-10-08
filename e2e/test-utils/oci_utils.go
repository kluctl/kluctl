package test_utils

import (
	"github.com/google/go-containerregistry/pkg/registry"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func CreateOciRegistry(t *testing.T, username string, password string) *url.URL {
	ociRegistry := registry.New()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if username != "" || password != "" {
			u, p, ok := r.BasicAuth()
			if !ok {
				w.Header().Add("WWW-Authenticate", "Basic")
				http.Error(w, "Auth header was incorrect", http.StatusUnauthorized)
				return
			}
			if u != username || p != password {
				http.Error(w, "Auth header was incorrect", http.StatusUnauthorized)
				return
			}
		}
		ociRegistry.ServeHTTP(w, r)
	}))

	t.Cleanup(s.Close)

	ociUrl := strings.ReplaceAll(s.URL, "http://", "oci://")
	ociUrl2, _ := url.Parse(ociUrl)

	return ociUrl2
}
