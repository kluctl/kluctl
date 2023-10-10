package auth_provider

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"strconv"
)

type OciEnvAuthProvider struct {
	Prefix string
}

func (a *OciEnvAuthProvider) isDefaultInsecure(ctx context.Context) bool {
	defaultInsecure, err := utils.ParseEnvBool(fmt.Sprintf("%s_DEFAULT_INSECURE", a.Prefix), false)
	if err != nil {
		status.Warningf(ctx, "Failed to parse %s_DEFAULT_INSECURE: %s", a.Prefix, err)
		return false
	}
	return defaultInsecure
}

func (a *OciEnvAuthProvider) Login(ctx context.Context, ociUrl string) (*OciAuthInfo, error) {
	defaultInsecure := a.isDefaultInsecure(ctx)

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
				RegistryToken: m["REGISTRY_TOKEN"],
			},
			Insecure: defaultInsecure,
		}
		if e.Registry == "" {
			continue
		}

		insecureStr, ok := m["INSECURE"]
		if ok {
			insecure, err := strconv.ParseBool(insecureStr)
			if err != nil {
				return nil, fmt.Errorf("failed to parse INSECURE from env: %w", err)
			}
			e.Insecure = insecure
		}

		la.AddEntry(e)
	}
	return la.Login(ctx, ociUrl)
}
