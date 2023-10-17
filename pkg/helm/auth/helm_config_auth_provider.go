package auth

import (
	"context"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/repo"
	"net/url"
	"strings"
)

type HelmConfigAuthProvider struct {
}

func (a *HelmConfigAuthProvider) FindAuthEntry(ctx context.Context, repoUrl url.URL, credentialsId string) (*repo.Entry, CleanupFunc, error) {
	env := cli.New()

	f, err := repo.LoadFile(env.RepositoryConfig)
	if err != nil {
		return nil, cleanupNoop, nil
	}

	repoUrl2 := strings.TrimSuffix(repoUrl.String(), "/")

	for _, e := range f.Repositories {
		x := strings.TrimSuffix(e.URL, "/")
		if repoUrl2 == x {
			return e, cleanupNoop, nil
		}
	}

	return nil, cleanupNoop, nil
}
