package auth_provider

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
)

type OciEnvAuthProvider struct {
	Prefix string
}

func (a *OciEnvAuthProvider) isDefaultInsecureSkipTlsVerify(ctx context.Context) bool {
	defaultInsecure, err := utils.ParseEnvBool(fmt.Sprintf("%s_DEFAULT_INSECURE_SKIP_TLS_VERIFY", a.Prefix), false)
	if err != nil {
		status.Warningf(ctx, "Failed to parse %s_DEFAULT_INSECURE_SKIP_TLS_VERIFY: %s", a.Prefix, err)
		return false
	}
	return defaultInsecure
}

func (a *OciEnvAuthProvider) FindAuthEntry(ctx context.Context, ociUrl string) (*AuthEntry, error) {
	defaultInsecureSkipTlsVerify := a.isDefaultInsecureSkipTlsVerify(ctx)

	var la ListAuthProvider

	for _, m := range utils.ParseEnvConfigSets(a.Prefix) {
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
		} else if repo == "" {
			repo = "*"
		}
		if host == "" {
			continue
		}

		e := AuthEntry{
			Registry: host,
			Repo:     repo,
			AuthConfig: authn.AuthConfig{
				Username:      m["USERNAME"],
				Password:      m["PASSWORD"],
				Auth:          m["AUTH"],
				IdentityToken: m["IDENTITY_TOKEN"],
				RegistryToken: m["TOKEN"],
			},
			InsecureSkipTlsVerify: defaultInsecureSkipTlsVerify,
		}
		if e.Registry == "" {
			continue
		}

		x, ok := m["PLAIN_HTTP"]
		if ok {
			e.PlainHTTP = utils.ParseBoolOrFalse(x)
		}

		x, ok = m["INSECURE_SKIP_TLS_VERIFY"]
		if ok {
			e.InsecureSkipTlsVerify = utils.ParseBoolOrFalse(x)
		}

		la.AddEntry(e)
	}
	return la.FindAuthEntry(ctx, ociUrl)
}
