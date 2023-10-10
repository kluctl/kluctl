package auth

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"helm.sh/helm/v3/pkg/repo"
	"net/url"
)

type HelmEnvAuthProvider struct {
	Prefix string
}

func (a *HelmEnvAuthProvider) isDefaultInsecure(ctx context.Context) bool {
	defaultInsecure, err := utils.ParseEnvBool(fmt.Sprintf("%s_DEFAULT_INSECURE", a.Prefix), false)
	if err != nil {
		status.Warningf(ctx, "Failed to parse %s_DEFAULT_INSECURE: %s", a.Prefix, err)
		return false
	}
	return defaultInsecure
}

func (a *HelmEnvAuthProvider) FindAuthEntry(ctx context.Context, repoUrl url.URL, credentialsId string) (*repo.Entry, CleanupFunc, error) {
	defaultInsecure := a.isDefaultInsecure(ctx)

	for _, m := range utils.ParseEnvConfigSets(a.Prefix) {
		host := m["HOST"]
		cid := m["CREDENTIALS_ID"]
		if host == "" && cid == "" {
			continue
		}

		e := AuthEntry{
			CredentialsId:         cid,
			Host:                  host,
			Path:                  m["PATH"],
			Username:              m["USERNAME"],
			Password:              m["PASSWORD"],
			InsecureSkipTLSverify: utils.ParseBoolOrDefault(m["INSECURE"], defaultInsecure),
			PassCredentialsAll:    utils.ParseBoolOrFalse(m["PASS_CREDENTIALS_ALL"]),
		}

		if !e.Match(repoUrl, credentialsId) {
			continue
		}

		ret, cf, err := e.BuildEntry(ctx, repoUrl)
		if err != nil {
			return nil, cf, err
		}
		if ret == nil {
			continue
		}

		ret.CertFile = m["CERT_FILE"]
		ret.KeyFile = m["KEY_FILE"]
		ret.CAFile = m["CA_FILE"]

		return ret, cf, nil
	}
	return nil, nil, nil
}
