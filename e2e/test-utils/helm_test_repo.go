package test_utils

import (
	"github.com/google/go-containerregistry/pkg/registry"
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

type TestHelmRepo struct {
	TestHttpServer

	Oci bool

	Path   string
	Charts []RepoChart

	URL url.URL
}

type RepoChart struct {
	ChartName string
	Version   string
}

func (s *TestHelmRepo) Start(t *testing.T) {
	if s.Oci {
		s.startOciRepo(t)
	} else {
		s.startHelmRepo(t)
	}
}

func (s *TestHelmRepo) startHelmRepo(t *testing.T) {
	tmpDir := t.TempDir()

	for _, c := range s.Charts {
		tgz := createHelmPackage(t, c.ChartName, c.Version)
		_ = cp.Copy(tgz, filepath.Join(tmpDir, s.Path, filepath.Base(tgz)))
	}

	fs := http.FileServer(http.FS(os.DirFS(tmpDir)))
	s.TestHttpServer.Start(t, fs)

	i, err := repo.IndexDirectory(tmpDir, s.Server.URL)
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

	u, _ := url.Parse(s.Server.URL + path)
	s.URL = *u
}

func (s *TestHelmRepo) buildOciRegistryClient(t *testing.T) *registry2.Client {
	var opts []registry2.ClientOption
	if !s.TLSEnabled {
		opts = append(opts, registry2.ClientOptPlainHTTP())
	}

	opts = append(opts, registry2.ClientOptHTTPClient(s.Server.Client()))

	if s.Password != "" {
		tmpConfigFile := filepath.Join(t.TempDir(), "config.json")
		opts = append(opts, registry2.ClientOptCredentialsFile(tmpConfigFile))
	}

	registryClient, err := registry2.NewClient(opts...)
	if err != nil {
		t.Fatal(err)
	}

	if s.Password != "" {
		var loginOpts []registry2.LoginOption
		loginOpts = append(loginOpts, registry2.LoginOptBasicAuth(s.Username, s.Password))
		if !s.TLSEnabled {
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

	s.TestHttpServer.Start(t, http.HandlerFunc(ociRegistry.ServeHTTP))

	u, _ := url.Parse(s.Server.URL)
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
