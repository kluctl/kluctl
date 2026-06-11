package auth

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sync"

	"github.com/gobwas/glob"
	"github.com/kluctl/kluctl/lib/envutils"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"helm.sh/helm/v4/pkg/repo/v1"
)

type HelmEnvAuthProvider struct {
	Prefix string

	mutext  sync.Mutex
	list    *ListAuthProvider
	listErr error
}

func (a *HelmEnvAuthProvider) isDefaultInsecureSkipTlsVerify(ctx context.Context) bool {
	defaultInsecure, err := envutils.ParseEnvBool(fmt.Sprintf("%s_DEFAULT_INSECURE_SKIP_TLS_VERIFY", a.Prefix), false)
	if err != nil {
		status.Warningf(ctx, "Failed to parse %s_DEFAULT_INSECURE_SKIP_TLS_VERIFY: %s", a.Prefix, err)
		return false
	}
	return defaultInsecure
}

func (a *HelmEnvAuthProvider) buildList(ctx context.Context) error {
	a.mutext.Lock()
	defer a.mutext.Unlock()
	if a.listErr != nil {
		return a.listErr
	}
	if a.list != nil {
		return nil
	}
	a.listErr = a.doBuildList(ctx)
	return a.listErr
}

func (a *HelmEnvAuthProvider) doBuildList(ctx context.Context) error {
	a.list = &ListAuthProvider{}

	defaultInsecure := a.isDefaultInsecureSkipTlsVerify(ctx)

	for _, s := range envutils.ParseEnvConfigSets(a.Prefix) {
		m := s.Map
		host := m["HOST"]
		if host == "" {
			continue
		}

		e := AuthEntry{
			Host:                  host,
			PathStr:               m["PATH"],
			Username:              m["USERNAME"],
			Password:              m["PASSWORD"],
			InsecureSkipTLSverify: utils.ParseBoolOrDefault(m["INSECURE_SKIP_TLS_VERIFY"], defaultInsecure),
			PassCredentialsAll:    utils.ParseBoolOrFalse(m["PASS_CREDENTIALS_ALL"]),
		}

		if e.PathStr != "" {
			g, err := glob.Compile(e.PathStr, '/')
			if err != nil {
				return err
			}
			e.PathGlob = g
		}

		p, ok := m["CERT_FILE"]
		if ok {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			e.Cert = b
		}
		p, ok = m["KEY_FILE"]
		if ok {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			e.Key = b
		}
		p, ok = m["CA_FILE"]
		if ok {
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			e.CA = b
		}

		a.list.AddEntry(e)
	}

	return nil
}

func (a *HelmEnvAuthProvider) FindAuthEntry(ctx context.Context, repoUrl url.URL) (*repo.Entry, CleanupFunc, error) {
	err := a.buildList(ctx)
	if err != nil {
		return nil, nil, err
	}
	return a.list.FindAuthEntry(ctx, repoUrl)
}
