package args

import (
	"context"
	"fmt"
	"github.com/gobwas/glob"
	"github.com/kluctl/kluctl/lib/status"
	helm_auth "github.com/kluctl/kluctl/v2/pkg/helm/auth"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"os"
	"strings"
)

type HelmCredentials struct {
	HelmUsername              []string `group:"helm" skipenv:"true" help:"Specify username to use for Helm Repository authentication. Must be in the form --helm-username=<host>/<path>=<username> or in the deprecated form --helm-username=<credentialsId>:<username>, where <credentialsId> must match the id specified in the helm-chart.yaml."`
	HelmPassword              []string `group:"helm" skipenv:"true" help:"Specify password to use for Helm Repository authentication. Must be in the form --helm-password=<host>/<path>=<password> or in the deprecated form --helm-password=<credentialsId>:<password>, where <credentialsId> must match the id specified in the helm-chart.yaml."`
	HelmKeyFile               []string `group:"helm" skipenv:"true" help:"Specify client certificate to use for Helm Repository authentication. Must be in the form --helm-key-file=<host>/<path>=<filePath> or in the deprecated form --helm-key-file=<credentialsId>:<filePath>, where <credentialsId> must match the id specified in the helm-chart.yaml."`
	HelmCertFile              []string `group:"helm" skipenv:"true" help:"Specify key to use for Helm Repository authentication. Must be in the form --helm-cert-file=<host>/<path>=<filePath> or in the deprecated form --helm-cert-file=<credentialsId>:<filePath>, where <credentialsId> must match the id specified in the helm-chart.yaml."`
	HelmCAFile                []string `group:"helm" skipenv:"true" help:"Specify ca bundle certificate to use for Helm Repository authentication. Must be in the form --helm-ca-file=<host>/<path>=<filePath> or in the deprecated form --helm-ca-file=<credentialsId>:<filePath>, where <credentialsId> must match the id specified in the helm-chart.yaml."`
	HelmInsecureSkipTlsVerify []string `group:"helm" skipenv:"true" help:"Controls skipping of TLS verification. Must be in the form --helm-insecure-skip-tls-verify=<host>/<path> or in the deprecated form --helm-insecure-skip-tls-verify=<credentialsId>, where <credentialsId> must match the id specified in the helm-chart.yaml."`
	HelmCreds                 []string `group:"helm" skipenv:"true" help:"This is a shortcut to --helm-username and --helm-password. Must be in the form --helm-creds=<host>/<path>=<username>:<password>, which specifies the username and password for the same repository."`
}

func (c *HelmCredentials) BuildAuthProvider(ctx context.Context) (helm_auth.HelmAuthProvider, error) {
	la := &helm_auth.ListAuthProvider{}
	if c == nil {
		return la, nil
	}

	var byCredentialId utils.OrderedMap[string, *helm_auth.AuthEntry]
	var byHostPath utils.OrderedMap[string, *helm_auth.AuthEntry]

	getDeprecatedEntry := func(s string) (*helm_auth.AuthEntry, string, bool) {
		x := strings.Split(s, ":")
		if len(x) != 2 {
			return nil, "", false
		}
		status.Deprecation(ctx, "helm-credential-args-id", "Passing Helm credentials via credentialsId is deprecated and support for it will be removed in a future version of Kluctl. Please switch to using the <host>/<path>=value format.")
		k := x[0]
		e, ok := byCredentialId.Get(k)
		if !ok {
			e = &helm_auth.AuthEntry{
				CredentialsId: k,
			}
			byCredentialId.Set(k, e)
		}
		return e, x[1], true
	}

	getEntry := func(s string, expectValue bool) (*helm_auth.AuthEntry, string, error) {
		if !strings.Contains(s, "=") {
			e, v, ok := getDeprecatedEntry(s)
			if ok {
				return e, v, nil
			}
		}

		x := strings.SplitN(s, "=", 2)
		if expectValue && len(x) != 2 {
			return nil, "", fmt.Errorf("expected value: %s", s)
		}

		k := x[0]
		e, ok := byHostPath.Get(k)
		if !ok {
			x := strings.SplitN(k, "/", 2)
			e = &helm_auth.AuthEntry{}
			if len(x) == 2 {
				e.Host = x[0]
				g, err := glob.Compile(x[1], '/')
				if err != nil {
					return nil, "", err
				}
				e.PathStr = x[1]
				e.PathGlob = g
			} else {
				e.Host = x[0]
			}
			byHostPath.Set(k, e)
		}

		if len(x) == 1 {
			return e, "", nil
		}
		return e, x[1], nil
	}

	for _, s := range c.HelmUsername {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		e.Username = v
	}
	for _, s := range c.HelmPassword {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		e.Password = v
	}
	for _, s := range c.HelmKeyFile {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		e.Key, err = os.ReadFile(v)
		if err != nil {
			return nil, err
		}
	}
	for _, s := range c.HelmCertFile {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		e.Cert, err = os.ReadFile(v)
		if err != nil {
			return nil, err
		}
	}
	for _, s := range c.HelmCAFile {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		e.CA, err = os.ReadFile(v)
		if err != nil {
			return nil, err
		}
	}
	for _, s := range c.HelmInsecureSkipTlsVerify {
		e, _, err := getEntry(s, false)
		if err != nil {
			return nil, err
		}
		e.InsecureSkipTLSverify = true
	}
	for _, s := range c.HelmCreds {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		x := strings.SplitN(v, ":", 2)
		if len(x) != 2 {
			return nil, fmt.Errorf("format of --helm-creds values must be <host>/<path>=<username>:<password>")
		}
		e.Username = x[0]
		e.Password = x[1]
	}

	for _, e := range byCredentialId.ListValues() {
		la.AddEntry(*e)
	}
	for _, e := range byHostPath.ListValues() {
		la.AddEntry(*e)
	}

	return la, nil
}
