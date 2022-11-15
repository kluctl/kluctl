package args

import (
	"strings"

	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/repo"
)

type HelmCredentials struct {
	HelmUsername              []string `group:"misc" help:"Specify username to use for Helm Repository authentication. Must be in the form --helm-username=<credentialsId>:<username>, where <credentialsId> must match the id specified in the helm-chart.yaml."`
	HelmPassword              []string `group:"misc" help:"Specify password to use for Helm Repository authentication. Must be in the form --helm-password=<credentialsId>:<password>, where <credentialsId> must match the id specified in the helm-chart.yaml."`
	HelmKeyFile               []string `group:"misc" help:"Specify client certificate to use for Helm Repository authentication. Must be in the form --helm-key-file=<credentialsId>:<path>, where <credentialsId> must match the id specified in the helm-chart.yaml."`
	HelmInsecureSkipTlsVerify []string `group:"misc" help:"Controls skipping of TLS verification. Must be in the form --helm-insecure-skip-tls-verify=<credentialsId>, where <credentialsId> must match the id specified in the helm-chart.yaml."`
}

func (c *HelmCredentials) FindCredentials(repoUrl string, credentialsId *string) *repo.Entry {
	if credentialsId != nil {
		splitIdAndValue := func(s string) (string, bool) {
			x := strings.SplitN(s, ":", 2)
			if len(x) < 0 {
				return "", false
			}
			if x[0] != *credentialsId {
				return "", false
			}
			return x[1], true
		}

		var e repo.Entry
		for _, x := range c.HelmUsername {
			if v, ok := splitIdAndValue(x); ok {
				e.Username = v
			}
		}
		for _, x := range c.HelmPassword {
			if v, ok := splitIdAndValue(x); ok {
				e.Password = v
			}
		}
		for _, x := range c.HelmKeyFile {
			if v, ok := splitIdAndValue(x); ok {
				e.KeyFile = v
			}
		}
		for _, x := range c.HelmInsecureSkipTlsVerify {
			if x == *credentialsId {
				e.InsecureSkipTLSverify = true
			}
		}

		if e != (repo.Entry{}) {
			e.URL = repoUrl
			return &e
		}
	}

	env := cli.New()

	f, err := repo.LoadFile(env.RepositoryConfig)
	if err != nil {
		return nil
	}

	removeTrailingSlash := func(s string) string {
		if len(s) == 0 {
			return s
		}
		if s[len(s)-1] == '/' {
			return s[:len(s)-1]
		}
		return s
	}
	repoUrl = removeTrailingSlash(repoUrl)

	for _, e := range f.Repositories {
		if removeTrailingSlash(e.URL) == repoUrl {
			return e
		}
	}

	return nil
}
