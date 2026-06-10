package test_utils

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-git/go-billy/v6/osfs"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/backend"
	config2 "github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/cache"
	"github.com/go-git/go-git/v6/storage"
	"github.com/go-git/go-git/v6/storage/filesystem"
	"github.com/huandu/xstrings"
	port_tool "github.com/kluctl/kluctl/v2/e2e/test-utils/port-tool"
	"sigs.k8s.io/yaml"
)

type TestGitServer struct {
	t *testing.T

	baseDir string

	backend       *backend.Backend
	gitHttpServer *http.Server
	gitServerPort int

	authUsername string
	authPassword string

	cleanupMutex  sync.RWMutex
	cleanupDoneCh chan struct{}
}

type TestGitServerOpt func(*TestGitServer)

func WithTestGitServerAuth(username string, password string) TestGitServerOpt {
	return func(server *TestGitServer) {
		server.authUsername = username
		server.authPassword = password
	}
}

func NewTestGitServer(t *testing.T, opts ...TestGitServerOpt) *TestGitServer {
	p := &TestGitServer{
		t:             t,
		baseDir:       t.TempDir(),
		cleanupDoneCh: make(chan struct{}),
	}

	for _, o := range opts {
		o(p)
	}

	p.initGitServer()

	t.Cleanup(func() {
		p.Cleanup()
	})

	return p
}

func (p *TestGitServer) initGitServer() {
	p.backend = backend.New(p)

	serveHttp := func(w http.ResponseWriter, r *http.Request) {
		if p.authUsername != "" {
			username, password, ok := r.BasicAuth()
			if !ok {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if p.authUsername != username || p.authPassword != password {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
		}

		p.backend.ServeHTTP(w, r)
	}

	p.gitHttpServer = &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: http.HandlerFunc(serveHttp),
	}

	ln := port_tool.NewListenerWithUniquePort("127.0.0.1")
	a := ln.Addr().(*net.TCPAddr)
	p.gitServerPort = a.Port

	go func() {
		err := p.gitHttpServer.Serve(ln)
		if err != nil {
			p.t.Logf("gitHttpServer.Serve() with port %d returned error: %s", p.gitServerPort, err.Error())
		} else {
			p.t.Logf("gitHttpServer.Serve() with port %d returned with no error", p.gitServerPort)
		}
		close(p.cleanupDoneCh)
	}()
}

func (p *TestGitServer) Load(u *url.URL) (storage.Storer, error) {
	pth := u.Path
	fs := osfs.New(filepath.Join(p.baseDir, pth))
	s := filesystem.NewStorage(fs, cache.NewObjectLRUDefault())
	return s, nil
}

func (p *TestGitServer) Cleanup() {
	p.cleanupMutex.Lock()
	defer p.cleanupMutex.Unlock()

	p.t.Logf("gitHttpServer.Cleanup() called for port %d", p.gitServerPort)

	if p.gitHttpServer != nil {
		_ = p.gitHttpServer.Shutdown(context.Background())
		p.gitHttpServer = nil
		p.backend = nil
		<-p.cleanupDoneCh
	}

	p.baseDir = ""
}

func (p *TestGitServer) GitInit(repo string) {
	gitDir := p.LocalGitDir(repo)
	workDir := p.LocalWorkDir(repo)

	err := os.MkdirAll(workDir, 0o700)
	if err != nil {
		p.t.Fatal(err)
	}

	r, err := git.PlainInit(workDir, false)
	if err != nil {
		p.t.Fatal(err)
	}
	err = os.Symlink(filepath.Join(workDir, ".git"), gitDir)
	if err != nil {
		p.t.Fatal(err)
	}

	_, err = r.CreateRemote(&config2.RemoteConfig{
		Name: "origin",
		URLs: []string{p.GitRepoUrl(repo)},
	})
	if err != nil {
		p.t.Fatal(err)
	}

	config, err := r.Config()
	if err != nil {
		p.t.Fatal(err)
	}

	config.User.Name = "Test User"
	config.User.Email = "no@mail.com"
	config.Author.Name = config.User.Name
	config.Author.Email = config.User.Email
	config.Committer.Name = config.User.Name
	config.Committer.Email = config.User.Email
	err = r.SetConfig(config)
	if err != nil {
		p.t.Fatal(err)
	}
	f, err := os.Create(filepath.Join(workDir, ".dummy"))
	if err != nil {
		p.t.Fatal(err)
	}
	_ = f.Close()

	wt, err := r.Worktree()
	if err != nil {
		p.t.Fatal(err)
	}
	_, err = wt.Add(".dummy")
	if err != nil {
		p.t.Fatal(err)
	}
	_, err = wt.Commit("initial", &git.CommitOptions{})
	if err != nil {
		p.t.Fatal(err)
	}
}

func (p *TestGitServer) CommitFiles(repo string, add []string, all bool, message string) plumbing.Hash {
	r, err := git.PlainOpen(p.LocalWorkDir(repo))
	if err != nil {
		p.t.Fatal(err)
	}
	wt, err := r.Worktree()
	if err != nil {
		p.t.Fatal(err)
	}
	for _, a := range add {
		_, err = wt.Add(a)
		if err != nil {
			p.t.Fatal(err)
		}
	}
	hash, err := wt.Commit(message, &git.CommitOptions{
		All: all,
	})
	if err != nil {
		p.t.Fatal(err)
	}
	return hash
}

func (p *TestGitServer) CommitFile(repo string, pth string, message string, content []byte) {
	fullPath := filepath.Join(p.LocalWorkDir(repo), pth)

	dir, _ := filepath.Split(fullPath)
	if dir != "" {
		err := os.MkdirAll(dir, 0o700)
		if err != nil {
			panic(err)
		}
	}

	err := os.WriteFile(fullPath, content, 0o600)
	if err != nil {
		p.t.Fatal(err)
	}
	if message == "" {
		message = fmt.Sprintf("update %s", filepath.Join(repo, pth))
	}
	p.CommitFiles(repo, []string{pth}, false, message)
}

func (p *TestGitServer) UpdateFile(repo string, pth string, update func(f string) (string, error), message string) {
	fullPath := filepath.Join(p.LocalWorkDir(repo), pth)
	f := ""
	if _, err := os.Stat(fullPath); err == nil {
		b, err := os.ReadFile(fullPath)
		if err != nil {
			p.t.Fatal(err)
		}
		f = string(b)
	}

	newF, err := update(f)
	if err != nil {
		p.t.Fatal(err)
	}

	if f == newF {
		return
	}
	err = os.MkdirAll(filepath.Dir(fullPath), 0o700)
	if err != nil {
		p.t.Fatal(err)
	}
	err = os.WriteFile(fullPath, []byte(newF), 0o600)
	if err != nil {
		p.t.Fatal(err)
	}
	p.CommitFiles(repo, []string{pth}, false, message)
}

func (p *TestGitServer) UpdateYaml(repo string, pth string, update func(o map[string]any) error, message string) {
	fullPath := filepath.Join(p.LocalWorkDir(repo), pth)

	var o map[string]any
	var origBytes []byte
	isNew := false
	if _, err := os.Stat(fullPath); err == nil {
		origBytes, err = os.ReadFile(fullPath)
		if err != nil {
			p.t.Fatal(err)
		}
		err = yaml.Unmarshal(origBytes, &o)
		if err != nil {
			p.t.Fatal(err)
		}
	} else {
		o = map[string]any{}
		isNew = true
	}

	err := update(o)
	if err != nil {
		p.t.Fatal(err)
	}

	newBytes, err := yaml.Marshal(o)
	if err != nil {
		p.t.Fatal(err)
	}
	if !isNew && bytes.Equal(origBytes, newBytes) {
		return
	}
	p.CommitFile(repo, pth, message, newBytes)
}

func (p *TestGitServer) DeleteFile(repo string, pth string, message string) {
	fullPath := filepath.Join(p.LocalWorkDir(repo), pth)
	_ = os.Remove(fullPath)

	if message == "" {
		message = fmt.Sprintf("delete %s", filepath.Join(repo, pth))
	}
	p.CommitFiles(repo, []string{pth}, false, message)
}

func (p *TestGitServer) ReadFile(repo string, pth string) []byte {
	fullPath := filepath.Join(p.LocalWorkDir(repo), pth)
	b, err := os.ReadFile(fullPath)
	if err != nil {
		p.t.Fatal(err)
	}
	return b
}

func (p *TestGitServer) GitHost() string {
	return fmt.Sprintf("localhost:%d", p.gitServerPort)
}

func (p *TestGitServer) GitUrl() string {
	return fmt.Sprintf("http://localhost:%d/%s", p.gitServerPort, p.testNameSlug())
}

func (p *TestGitServer) GitRepoUrl(repo string) string {
	return fmt.Sprintf("%s/%s", p.GitUrl(), repo)
}

func (p *TestGitServer) testNameSlug() string {
	n := xstrings.ToKebabCase(p.t.Name())
	n = strings.ReplaceAll(n, "/", "-")
	return n
}

func (p *TestGitServer) LocalGitDir(repo string) string {
	return filepath.Join(p.baseDir, p.testNameSlug(), repo)
}

func (p *TestGitServer) LocalWorkDir(repo string) string {
	return filepath.Join(p.baseDir, p.testNameSlug(), repo) + "-workdir"
}

func (p *TestGitServer) GetGitRepo(repo string) *git.Repository {
	r, err := git.PlainOpen(p.LocalWorkDir(repo))
	if err != nil {
		p.t.Fatal(err)
	}
	return r
}

func (p *TestGitServer) GetWorktree(repo string) *git.Worktree {
	r := p.GetGitRepo(repo)
	wt, err := r.Worktree()
	if err != nil {
		p.t.Fatal(err)
	}
	return wt
}
