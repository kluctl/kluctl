package auth_provider

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"strings"
)

type OciDockerConfigAuthProvider struct {
}

func (o OciDockerConfigAuthProvider) Login(ctx context.Context, ociUrl string) (*OciAuthInfo, error) {
	if !strings.HasPrefix(ociUrl, "oci://") {
		return nil, fmt.Errorf("invalid oci url: %s", ociUrl)
	}

	ociRef, err := name.ParseReference(strings.TrimPrefix(ociUrl, "oci://"))
	if err != nil {
		return nil, err
	}

	auth, err := authn.DefaultKeychain.Resolve(ociRef.Context())
	status.Infof(ctx, "login ociRef=%s, auth=%v, err=%s", ociRef.String(), auth, err)
	if err != nil {
		return nil, err
	}
	if auth == authn.Anonymous {
		return nil, nil
	}

	return &OciAuthInfo{
		Authenticator: auth,
	}, nil
}
