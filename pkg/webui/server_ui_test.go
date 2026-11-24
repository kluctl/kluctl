package webui

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/gin-gonic/gin"
	"github.com/kluctl/kluctl/v2/pkg/results"
	"github.com/kluctl/kluctl/v2/pkg/types/result"
	"github.com/stretchr/testify/require"
)

func TestWebUIStaticLoginWithPrefix(t *testing.T) {
	auth := &authHandler{
		authConfig:     AuthConfig{AuthEnabled: true, StaticLoginEnabled: true},
		authSecret:     []byte("test-session-key"),
		viewerPassword: "test-viewer-password",
		oidcProvider:   &oidc.Provider{},
	}
	engine := gin.New()
	group := engine.Group("/tools/kluctl")
	require.NoError(t, auth.setupRoutes(group, false))
	if IsWebUiBuildIncluded() {
		require.NoError(t, (&CommandResultsServer{}).setupStaticRoutes(group))
	}
	server := httptest.NewServer(engine)
	defer server.Close()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	base := server.URL + "/tools/kluctl/"

	get := func(path string, status int) *http.Response {
		t.Helper()
		r, err := client.Get(base + path)
		require.NoError(t, err)
		t.Cleanup(func() { _ = r.Body.Close() })
		require.Equal(t, status, r.StatusCode)
		return r
	}
	if IsWebUiBuildIncluded() {
		get("", http.StatusOK)
		get("staticbuild.js", http.StatusOK)
	}
	get("auth/user", http.StatusUnauthorized)
	callback := get("auth/callback?state=mismatch", http.StatusTemporaryRedirect)
	location, err := url.Parse(callback.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "/tools/kluctl/", location.Path)
	require.Equal(t, "invalid state parameter", location.Query().Get("login_error"))
	r, err := client.Post(base+"auth/staticLogin", "application/json", strings.NewReader(`{"username":"viewer","password":"test-viewer-password"}`))
	require.NoError(t, err)
	_ = r.Body.Close()
	require.Equal(t, http.StatusOK, r.StatusCode)
	user := get("auth/user", http.StatusOK)
	var viewer User
	require.NoError(t, json.NewDecoder(user.Body).Decode(&viewer))
	require.Equal(t, User{Username: "viewer", IsAdmin: false}, viewer)
	logout := get("auth/logout", http.StatusTemporaryRedirect)
	require.Equal(t, "/tools/kluctl/", logout.Header.Get("Location"))
	get("auth/user", http.StatusUnauthorized)
}

type snapshotFixture struct{ results.ResultStore }

func (snapshotFixture) ListCommandResultSummaries(results.ListResultSummariesOptions) ([]result.CommandResultSummary, error) {
	return []result.CommandResultSummary{{Id: "cli-run"}}, nil
}
func (snapshotFixture) ListKluctlDeployments() ([]results.WatchKluctlDeploymentEvent, error) {
	return nil, nil
}
func (snapshotFixture) GetCommandResult(opts results.GetCommandResultOptions) (*result.CommandResult, error) {
	return &result.CommandResult{Id: opts.Id}, nil
}

func TestWebUISnapshotWithoutGitOpsTargets(t *testing.T) {
	if !IsWebUiBuildIncluded() {
		t.Skip("run make build-webui to test the embedded build")
	}
	dest := t.TempDir()
	require.NoError(t, NewStaticWebuiBuilder(snapshotFixture{}, 1).Build(context.Background(), dest))
	read := func(path string) string {
		t.Helper()
		b, err := os.ReadFile(filepath.Join(dest, path))
		require.NoError(t, err)
		return string(b)
	}
	require.Contains(t, read("index.html"), `src="./staticbuild.js"`)
	require.Contains(t, read("staticbuild.js"), "isStaticBuild = true")
	require.Contains(t, read("staticdata/summaries.js"), "cli-run")
	require.Contains(t, read("staticdata/result-cli-run.js"), `staticResults.set("cli-run",`)
	require.Contains(t, read("staticdata/kluctldeployments.js"), "staticKluctlDeployments=null")
}
