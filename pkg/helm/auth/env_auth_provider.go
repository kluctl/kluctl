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

func (a *HelmEnvAuthProvider) isDefaultInsecureSkipTlsVerify(ctx context.Context) bool {
	defaultInsecure, err := utils.ParseEnvBool(fmt.Sprintf("%s_DEFAULT_INSECURE_SKIP_TLS_VERIFY", a.Prefix), false)
	if err != nil {
		status.Warningf(ctx, "Failed to parse %s_DEFAULT_INSECURE_SKIP_TLS_VERIFY: %s", a.Prefix, err)
		return false
	}
	return defaultInsecure
}

func (a *HelmEnvAuthProvider) FindAuthEntry(ctx context.Context, repoUrl url.URL, credentialsId string) (*repo.Entry, CleanupFunc, error) {
	defaultInsecure := a.isDefaultInsecureSkipTlsVerify(ctx)

	for _, s := range utils.ParseEnvConfigSets(a.Prefix) {
		m := s.Map
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
			InsecureSkipTLSverify: utils.ParseBoolOrDefault(m["INSECURE_SKIP_TLS_VERIFY"], defaultInsecure),
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
