package test_utils

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/kluctl/kluctl/lib/git/types"
	cp "github.com/otiai10/copy"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/pusher"
	registry2 "helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
	"helm.sh/helm/v3/pkg/uploader"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type TestHelmRepoType int

const (
	TestHelmRepo_Helm TestHelmRepoType = iota
	TestHelmRepo_Oci
	TestHelmRepo_Git
	TestHelmRepo_Local
)

type TestHelmRepo struct {
	HttpServer TestHttpServer
	GitServer  *TestGitServer

	Type TestHelmRepoType

	Path   string
	Charts []RepoChart

	URL url.URL
}

type RepoChart struct {
	ChartName string
	Version   string
}

func NewHelmTestRepo(helmType TestHelmRepoType, path string, charts []RepoChart) *TestHelmRepo {
	return &TestHelmRepo{
		Type:   helmType,
		Path:   path,
		Charts: charts,
	}
}

func NewHelmTestRepoGit(t *testing.T, path string, charts []RepoChart, username string, password string) *TestHelmRepo {
	r := NewHelmTestRepo(TestHelmRepo_Git, path, charts)
	r.GitServer = NewTestGitServer(t, WithTestGitServerAuth(username, password))
	return r
}

func NewHelmTestRepoLocal(path string) *TestHelmRepo {
	return &TestHelmRepo{
		Type: TestHelmRepo_Local,
		Path: path,
	}
}

func (s *TestHelmRepo) Start(t *testing.T) {
	switch s.Type {
	case TestHelmRepo_Helm:
		s.startHelmRepo(t)
	case TestHelmRepo_Oci:
		s.startOciRepo(t)
	case TestHelmRepo_Git:
		s.startGitRepo(t)
	}
}

func (s *TestHelmRepo) startHelmRepo(t *testing.T) {
	tmpDir := t.TempDir()

	for _, c := range s.Charts {
		tgz := createHelmPackage(t, c.ChartName, c.Version)
		_ = cp.Copy(tgz, filepath.Join(tmpDir, s.Path, filepath.Base(tgz)))
	}

	fs := http.FileServer(http.FS(os.DirFS(tmpDir)))
	s.HttpServer.Start(t, fs)

	i, err := repo.IndexDirectory(tmpDir, s.HttpServer.Server.URL)
	if err != nil {
		t.Fatal(err)
	}

	i.SortEntries()
	err = i.WriteFile(filepath.Join(tmpDir, s.Path, "index.yaml"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	path := s.Path
	if path != "" {
		path = "/" + path
	}

	u, _ := url.Parse(s.HttpServer.Server.URL + path)
	s.URL = *u
}

func (s *TestHelmRepo) buildOciRegistryClient(t *testing.T) *registry2.Client {
	var opts []registry2.ClientOption
	if !s.HttpServer.TLSEnabled {
		opts = append(opts, registry2.ClientOptPlainHTTP())
	}

	opts = append(opts, registry2.ClientOptHTTPClient(s.HttpServer.Server.Client()))

	if s.HttpServer.Password != "" {
		tmpConfigFile := filepath.Join(t.TempDir(), "config.json")
		opts = append(opts, registry2.ClientOptCredentialsFile(tmpConfigFile))
	}

	registryClient, err := registry2.NewClient(opts...)
	if err != nil {
		t.Fatal(err)
	}

	if s.HttpServer.Password != "" {
		var loginOpts []registry2.LoginOption
		loginOpts = append(loginOpts, registry2.LoginOptBasicAuth(s.HttpServer.Username, s.HttpServer.Password))
		if !s.HttpServer.TLSEnabled {
			loginOpts = append(loginOpts, registry2.LoginOptInsecure(true))
		}
		err = registryClient.Login(s.URL.Host, loginOpts...)
		if err != nil {
			t.Fatal(err)
		}
	}
	return registryClient
}

func (s *TestHelmRepo) startOciRepo(t *testing.T) {
	tmpDir := t.TempDir()

	ociRegistry := registry.New()

	s.HttpServer.Start(t, http.HandlerFunc(ociRegistry.ServeHTTP))

	u, _ := url.Parse(s.HttpServer.Server.URL)
	s.URL = *u
	s.URL.Scheme = "oci"

	var out strings.Builder
	settings := cli.New()
	c := uploader.ChartUploader{
		Out:     &out,
		Pushers: pusher.All(settings),
		Options: []pusher.Option{},
	}

	registryClient := s.buildOciRegistryClient(t)
	c.Options = append(c.Options, pusher.WithRegistryClient(registryClient))

	for _, chart := range s.Charts {
		tgz := createHelmPackage(t, chart.ChartName, chart.Version)
		_ = cp.Copy(tgz, filepath.Join(tmpDir, filepath.Base(tgz)))

		err := c.UploadTo(tgz, s.URL.String())
		if err != nil {
			t.Fatal(err)
		}
	}

	if registryClient != nil {
		registryClient.Logout(s.URL.Host)
	}
}

func (s *TestHelmRepo) startGitRepo(t *testing.T) {
	s.GitServer.GitInit("helm-repo")

	for _, c := range s.Charts {
		dir := s.GitServer.LocalWorkDir("helm-repo")
		dir = filepath.Join(dir, s.Path, c.ChartName)
		_ = os.MkdirAll(dir, 0700)
		CreateHelmDir(t, c.ChartName, c.Version, dir)
		msg := fmt.Sprintf("chart %s version %s", c.ChartName, c.Version)
		hash := s.GitServer.CommitFiles("helm-repo", []string{c.ChartName}, false, msg)
		_, err := s.GitServer.GetGitRepo("helm-repo").CreateTag(c.Version, hash, &git.CreateTagOptions{
			Message: msg,
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	u := types.ParseGitUrlMust(s.GitServer.GitRepoUrl("helm-repo"))
	s.URL = u.URL
}
