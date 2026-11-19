package auth_provider

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kluctl/kluctl/lib/status"
)

type OciDockerConfigAuthProvider struct {
}

type exactRepository string

func (r exactRepository) Match(repo string) bool {
	return string(r) == repo
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
	status.Tracef(ctx, "login ociRef=%s, auth=%v, err=%v", ociRef.String(), auth, err)
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

	repo := ociRef.Context().RepositoryStr()
	return &AuthEntry{
		Registry:   ociRef.Context().RegistryStr(),
		RepoStr:    repo,
		RepoGlob:   exactRepository(repo),
		AuthConfig: *authConfig,
	}, nil
}
