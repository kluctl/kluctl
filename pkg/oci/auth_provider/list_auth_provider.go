package auth_provider

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"path"
	"strings"
)

type ListAuthProvider struct {
	entries []AuthEntry
}

func (a *ListAuthProvider) AddEntry(e AuthEntry) {
	a.entries = append(a.entries, e)
}

func (a *ListAuthProvider) FindAuthEntry(ctx context.Context, ociUrl string) (*AuthEntry, error) {
	status.Tracef(ctx, "ListAuthProvider: BuildAuth for %s", ociUrl)

	if !strings.HasPrefix(ociUrl, "oci://") {
		return nil, fmt.Errorf("invalid oci url: %s", ociUrl)
	}

	ociRef, err := name.ParseReference(strings.TrimPrefix(ociUrl, "oci://"))
	if err != nil {
		return nil, err
	}

	repo := ociRef.Context().RepositoryStr()

	for _, e := range a.entries {
		status.Tracef(ctx, "ListAuthProvider: try registry=%s, repo=%s", e.Registry, e.Repo)

		if e.Registry != ociRef.Context().RegistryStr() {
			continue
		}

		if e.Repo != "" {
			if m, _ := path.Match(e.Repo, repo); !m {
				continue
			}
		}

		return &e, nil
	}
	return nil, nil
}
