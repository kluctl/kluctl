package auth_provider

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"strings"
)

type ListAuthProvider struct {
	entries []AuthEntry
}

type AuthEntry struct {
	Registry string
	Repo     string

	AuthConfig authn.AuthConfig

	Insecure bool
}

func (a *ListAuthProvider) AddEntry(e AuthEntry) {
	a.entries = append(a.entries, e)
}

func splitRepo(repoIn string) (string, string) {
	var org, repo string
	s := strings.SplitN(repoIn, "/", 2)
	if len(s) == 1 {
		repo = s[0]
	} else {
		org = s[0]
		repo = s[1]
	}
	return org, repo
}

func (a *ListAuthProvider) Login(ctx context.Context, ociUrl string) (*OciAuthInfo, error) {
	status.Tracef(ctx, "ListAuthProvider: BuildAuth for %s", ociUrl)

	if !strings.HasPrefix(ociUrl, "oci://") {
		return nil, fmt.Errorf("invalid oci url: %s", ociUrl)
	}

	ociRef, err := name.ParseReference(strings.TrimPrefix(ociUrl, "oci://"))
	if err != nil {
		return nil, err
	}

	org, repo := splitRepo(ociRef.Context().RepositoryStr())

	for _, e := range a.entries {
		status.Tracef(ctx, "ListAuthProvider: try registry=%s, repo=%s", e.Registry, e.Repo)

		if e.Registry != ociRef.Context().RegistryStr() {
			continue
		}

		testOrg, testRepo := splitRepo(e.Repo)
		if testOrg == "" {
			testOrg = "*"
		}

		if testOrg != "*" && testOrg != org {
			continue
		}
		if testRepo != "*" && testRepo != repo {
			continue
		}

		return &OciAuthInfo{
			Authenticator: authn.FromConfig(e.AuthConfig),
			Insecure:      e.Insecure,
		}, nil
	}
	return nil, nil
}
