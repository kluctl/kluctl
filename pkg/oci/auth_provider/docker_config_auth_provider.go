package auth_provider

import (
	"context"
	"fmt"
	"github.com/gobwas/glob/match"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kluctl/kluctl/v2/lib/status"
	"strings"
)

type OciDockerConfigAuthProvider struct {
}

func (o OciDockerConfigAuthProvider) FindAuthEntry(ctx context.Context, ociUrl string) (*AuthEntry, error) {
	if !strings.HasPrefix(ociUrl, "oci://") {
		return nil, fmt.Errorf("invalid oci url: %s", ociUrl)
	}

	ociRef, err := name.ParseReference(strings.TrimPrefix(ociUrl, "oci://"))
	if err != nil {
		return nil, err
	}

	auth, err := authn.DefaultKeychain.Resolve(ociRef.Context())
	status.Tracef(ctx, "login ociRef=%s, auth=%v, err=%s", ociRef.String(), auth, err)
	if err != nil {
		return nil, err
	}
	if auth == authn.Anonymous {
		return nil, nil
	}

	authConfig, err := auth.Authorization()
	if err != nil {
		return nil, err
	}

	g := match.NewText(ociRef.Context().RepositoryStr())
	return &AuthEntry{
		Registry:   ociRef.Context().RegistryStr(),
		RepoStr:    ociRef.Context().RepositoryStr(),
		RepoGlob:   g,
		AuthConfig: *authConfig,
	}, nil
}
