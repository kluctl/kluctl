package auth

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"helm.sh/helm/v3/pkg/repo"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type ListAuthProvider struct {
	entries []AuthEntry
}

type AuthEntry struct {
	CredentialsId string

	Host string
	Path string

	Username string
	Password string

	Cert []byte
	Key  []byte
	CA   []byte

	InsecureSkipTLSverify bool
	PassCredentialsAll    bool
}

func (e *AuthEntry) Match(repoUrl url.URL, credentialsId string) bool {
	if e.CredentialsId != "" {
		if credentialsId == "" {
			return false
		}
		if e.CredentialsId != credentialsId {
			return false
		}
		return true
	}

	if e.Host != repoUrl.Host {
		return false
	}

	if e.Path != "" {
		urlPath := strings.TrimPrefix(repoUrl.Path, "/")
		m, err := filepath.Match(e.Path, urlPath)
		if err != nil || !m {
			return false
		}
	}

	return true
}

func (e *AuthEntry) BuildEntry(ctx context.Context, repoUrl url.URL) (*repo.Entry, CleanupFunc, error) {
	var tmpFiles []string
	cleanup := func() {
		for _, f := range tmpFiles {
			_ = os.Remove(f)
		}
	}
	newTmpFile := func(data []byte) (string, error) {
		f, err := os.CreateTemp(utils.GetTmpBaseDir(ctx), "")
		if err != nil {
			return "", err
		}
		defer f.Close()
		tmpFiles = append(tmpFiles, f.Name())

		_, err = f.Write(data)
		if err != nil {
			return "", err
		}
		return f.Name(), nil
	}

	ret := repo.Entry{
		URL:                   repoUrl.String(),
		Username:              e.Username,
		Password:              e.Password,
		InsecureSkipTLSverify: e.InsecureSkipTLSverify,
		PassCredentialsAll:    e.PassCredentialsAll,
	}
	var err error
	if e.Cert != nil {
		ret.CertFile, err = newTmpFile(e.Cert)
		if err != nil {
			cleanup()
			return nil, cleanupNoop, err
		}
	}
	if e.Key != nil {
		ret.KeyFile, err = newTmpFile(e.Key)
		if err != nil {
			cleanup()
			return nil, cleanupNoop, err
		}
	}
	if e.CA != nil {
		ret.CAFile, err = newTmpFile(e.CA)
		if err != nil {
			cleanup()
			return nil, cleanupNoop, err
		}
	}

	return &ret, cleanup, nil
}

func (a *ListAuthProvider) AddEntry(e AuthEntry) {
	a.entries = append(a.entries, e)
}

func (a *ListAuthProvider) FindAuthEntry(ctx context.Context, repoUrl url.URL, credentialsId string) (*repo.Entry, CleanupFunc, error) {
	status.Tracef(ctx, "ListAuthProvider: BuildAuth for %s and credentialsId %v", repoUrl.String(), credentialsId)

	for _, e := range a.entries {
		status.Tracef(ctx, "ListAuthProvider: try host=%s, path=%s", e.Host, e.Path)

		if !e.Match(repoUrl, credentialsId) {
			status.Tracef(ctx, "ListAuthProvider: no match")
			continue
		}

		status.Tracef(ctx, "ListAuthProvider: matched")
		return e.BuildEntry(ctx, repoUrl)
	}
	return nil, cleanupNoop, nil
}
