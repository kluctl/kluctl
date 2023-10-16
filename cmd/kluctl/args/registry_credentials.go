package args

import (
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"os"
	"strings"
)

type RegistryCredentials struct {
	RegistryUsername      []string `group:"registry" help:"Specify username to use for OCI authentication. Must be in the form --registry-username=<registry>/<repo>=<username>."`
	RegistryPassword      []string `group:"registry" help:"Specify password to use for OCI authentication. Must be in the form --registry-password=<registry>/<repo>=<password>."`
	RegistryAuth          []string `group:"registry" help:"Specify auth string to use for OCI authentication. Must be in the form --registry-auth=<registry>/<repo>=<auth>."`
	RegistryIdentityToken []string `group:"registry" help:"Specify identity token to use for OCI authentication. Must be in the form --registry-identity-token=<registry>/<repo>=<identity-token>."`
	RegistryToken         []string `group:"registry" help:"Specify registry token to use for OCI authentication. Must be in the form --registry-token=<registry>/<repo>=<token>."`
	RegistryCreds         []string `group:"registry" help:"This is a shortcut to --registry-username, --registry-password and --registry-token. It can be specified in two different forms. The first one is --registry-creds=<registry>/<repo>=<username>:<password>, which specifies the username and password for the same registry. The second form is --registry-creds=<registry>/<repo>=<token>, which specifies a JWT token for the specified registry."`

	RegistryKeyFile  []string `group:"registry" help:"Specify key to use for OCI authentication. Must be in the form --registry-key-file=<registry>/<repo>=<filePath>."`
	RegistryCertFile []string `group:"registry" help:"Specify certificate to use for OCI authentication. Must be in the form --registry-cert-file=<registry>/<repo>=<filePath>."`
	RegistryCAFile   []string `group:"registry" help:"Specify CA bundle to use for https verification. Must be in the form --registry-ca-file=<registry>/<repo>=<filePath>."`

	RegistryPlainHttp             []string `group:"registry" help:"Forces the use of http (no TLS). Must be in the form --registry-plain-http=<registry>/<repo>."`
	RegistryInsecureSkipTlsVerify []string `group:"registry" help:"Controls skipping of TLS verification. Must be in the form --registry-insecure-skip-tls-verify=<registry>/<repo>."`
}

func (c *RegistryCredentials) BuildAuthProvider(ctx context.Context) (auth_provider.OciAuthProvider, error) {
	la := &auth_provider.ListAuthProvider{}
	if c == nil {
		return la, nil
	}

	var byRegistryAndRepo utils.OrderedMap[*auth_provider.AuthEntry]

	getEntry := func(s string, expectValue bool) (*auth_provider.AuthEntry, string, error) {
		x := strings.SplitN(s, "=", 2)
		if expectValue && len(x) != 2 {
			return nil, "", fmt.Errorf("expected value")
		}
		k := x[0]
		e, ok := byRegistryAndRepo.Get(k)
		if !ok {
			x := strings.SplitN(k, "/", 2)
			e = &auth_provider.AuthEntry{}
			if len(x) == 2 {
				e.Registry = x[0]
				e.Repo = x[1]
			} else {
				e.Registry = x[0]
			}
			byRegistryAndRepo.Set(k, e)
		}

		if len(x) == 1 {
			return e, "", nil
		}
		return e, x[1], nil
	}

	for _, s := range c.RegistryUsername {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		e.AuthConfig.Username = v
	}
	for _, s := range c.RegistryPassword {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		e.AuthConfig.Password = v
	}
	for _, s := range c.RegistryAuth {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		e.AuthConfig.Auth = v
	}
	for _, s := range c.RegistryIdentityToken {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		e.AuthConfig.IdentityToken = v
	}
	for _, s := range c.RegistryToken {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		e.AuthConfig.RegistryToken = v
	}
	for _, s := range c.RegistryKeyFile {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(v)
		if err != nil {
			return nil, err
		}
		e.Key = b
	}
	for _, s := range c.RegistryCertFile {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(v)
		if err != nil {
			return nil, err
		}
		e.Cert = b
	}
	for _, s := range c.RegistryCAFile {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(v)
		if err != nil {
			return nil, err
		}
		e.CA = b
	}
	for _, s := range c.RegistryPlainHttp {
		e, _, err := getEntry(s, false)
		if err != nil {
			return nil, err
		}
		e.PlainHTTP = true
	}
	for _, s := range c.RegistryInsecureSkipTlsVerify {
		e, _, err := getEntry(s, false)
		if err != nil {
			return nil, err
		}
		e.InsecureSkipTlsVerify = true
	}
	for _, s := range c.RegistryCreds {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		x := strings.SplitN(v, ":", 2)
		if len(x) == 1 {
			e.AuthConfig.RegistryToken = x[0]
		} else {
			e.AuthConfig.Username = x[0]
			e.AuthConfig.Password = x[1]
		}
	}

	for _, e := range byRegistryAndRepo.ListValues() {
		la.AddEntry(*e)
	}

	return la, nil
}
