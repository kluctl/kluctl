package auth_provider

import (
	"context"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kluctl/kluctl/lib/envutils"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"os"
	"sync"
)

type OciEnvAuthProvider struct {
	Prefix string

	mutext  sync.Mutex
	list    *ListAuthProvider
	listErr error
}

func (a *OciEnvAuthProvider) isDefaultInsecureSkipTlsVerify(ctx context.Context) bool {
	defaultInsecure, err := envutils.ParseEnvBool(fmt.Sprintf("%s_DEFAULT_INSECURE_SKIP_TLS_VERIFY", a.Prefix), false)
	if err != nil {
		status.Warningf(ctx, "Failed to parse %s_DEFAULT_INSECURE_SKIP_TLS_VERIFY: %s", a.Prefix, err)
		return false
	}
	return defaultInsecure
}

func (a *OciEnvAuthProvider) buildList(ctx context.Context) error {
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

func (a *OciEnvAuthProvider) doBuildList(ctx context.Context) error {
	a.list = &ListAuthProvider{}

	defaultInsecureSkipTlsVerify := a.isDefaultInsecureSkipTlsVerify(ctx)

	for _, s := range envutils.ParseEnvConfigSets(a.Prefix) {
		m := s.Map
		host := m["HOST"]
		repo := m["REPOSITORY"]
		if host == "" && repo == "" {
			continue
		}

		if host == "" {
			ociRef, err := name.ParseReference(repo)
			if err != nil {
				continue
			}
			host = ociRef.Context().RegistryStr()
			repo = ociRef.Context().RepositoryStr()
		}
		if host == "" {
			continue
		}

		e := AuthEntry{
			Registry: host,
			AuthConfig: authn.AuthConfig{
				Username:      m["USERNAME"],
				Password:      m["PASSWORD"],
				IdentityToken: m["IDENTITY_TOKEN"],
				RegistryToken: m["TOKEN"],
			},
			InsecureSkipTlsVerify: defaultInsecureSkipTlsVerify,
		}
		if e.Registry == "" {
			continue
		}

		if repo != "" {
			g, err := glob.Compile(repo, '/')
			if err != nil {
				return err
			}
			e.RepoStr = repo
			e.RepoGlob = g
		}

		x, ok := m["PLAIN_HTTP"]
		if ok {
			e.PlainHTTP = utils.ParseBoolOrFalse(x)
		}

		x, ok = m["INSECURE_SKIP_TLS_VERIFY"]
		if ok {
			e.InsecureSkipTlsVerify = utils.ParseBoolOrFalse(x)
		}

		x, ok = m["CA_FILE"]
		if ok {
			b, err := os.ReadFile(x)
			if err != nil {
				return err
			}
			e.CA = b
		}
		x, ok = m["CERT_FILE"]
		if ok {
			b, err := os.ReadFile(x)
			if err != nil {
				return err
			}
			e.Cert = b
		}
		x, ok = m["KEY_FILE"]
		if ok {
			b, err := os.ReadFile(x)
			if err != nil {
				return err
			}
			e.Key = b
		}

		a.list.AddEntry(e)
	}

	return nil
}

func (a *OciEnvAuthProvider) FindAuthEntry(ctx context.Context, ociUrl string) (*AuthEntry, error) {
	err := a.buildList(ctx)
	if err != nil {
		return nil, err
	}
	return a.list.FindAuthEntry(ctx, ociUrl)
}
