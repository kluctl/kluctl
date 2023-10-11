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
	RegistryUsername      []string `group:"misc" help:"Specify username to use for OCI authentication. Must be in the form --registry-username=<registry>/<repo>=<username>."`
	RegistryPassword      []string `group:"misc" help:"Specify password to use for OCI authentication. Must be in the form --registry-password=<registry>/<repo>=<password>."`
	RegistryAuth          []string `group:"misc" help:"Specify auth string to use for OCI authentication. Must be in the form --registry-auth=<registry>/<repo>=<auth>."`
	RegistryIdentityToken []string `group:"misc" help:"Specify auth string to use for OCI authentication. Must be in the form --registry-identity-token=<registry>/<repo>=<identity-token>."`
	RegistryToken         []string `group:"misc" help:"Specify auth string to use for OCI authentication. Must be in the form --registry-token=<registry>/<repo>=<token>."`

	RegistryKeyFile  []string `group:"misc" help:"Specify key to use for OCI authentication. Must be in the form --registry-key-file=<registry>/<repo>=<filePath>."`
	RegistryCertFile []string `group:"misc" help:"Specify certificate to use for OCI authentication. Must be in the form --registry-cert-file=<registry>/<repo>=<filePath>."`
	RegistryCAFile   []string `group:"misc" help:"Specify CA bundle to use for https verification. Must be in the form --registry-ca-file=<registry>/<repo>=<filePath>."`

	RegistryInsecureSkipTlsVerify []string `group:"misc" help:"Controls skipping of TLS verification. Must be in the form --registry-insecure-skip-tls-verify=<registry>/<repo>."`
}

func (c *RegistryCredentials) BuildAuthProvider(ctx context.Context) (auth_provider.OciAuthProvider, error) {
	la := &auth_provider.ListAuthProvider{}
	if c == nil {
		return la, nil
	}

	byRegistryAndRepo := map[string]*auth_provider.AuthEntry{}

	getEntry := func(s string) (*auth_provider.AuthEntry, string, error) {
		x := strings.SplitN(s, "=", 2)
		if len(x) == 2 {
			k := x[0]
			e, ok := byRegistryAndRepo[k]
			if !ok {
				x := strings.SplitN(k, "/", 2)
				e = &auth_provider.AuthEntry{}
				if len(x) == 2 {
					e.Registry = x[0]
					e.Repo = x[1]
				} else {
					e.Registry = x[0]
				}
				byRegistryAndRepo[k] = e
			}
			return e, x[1], nil
		} else {
			return nil, "", fmt.Errorf("invalid parameter format")
		}
	}

	for _, s := range c.RegistryUsername {
		e, v, err := getEntry(s)
		if err != nil {
			return nil, err
		}
		e.AuthConfig.Username = v
	}
	for _, s := range c.RegistryPassword {
		e, v, err := getEntry(s)
		if err != nil {
			return nil, err
		}
		e.AuthConfig.Password = v
	}
	for _, s := range c.RegistryAuth {
		e, v, err := getEntry(s)
		if err != nil {
			return nil, err
		}
		e.AuthConfig.Auth = v
	}
	for _, s := range c.RegistryIdentityToken {
		e, v, err := getEntry(s)
		if err != nil {
			return nil, err
		}
		e.AuthConfig.IdentityToken = v
	}
	for _, s := range c.RegistryToken {
		e, v, err := getEntry(s)
		if err != nil {
			return nil, err
		}
		e.AuthConfig.RegistryToken = v
	}
	for _, s := range c.RegistryKeyFile {
		e, v, err := getEntry(s)
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
		e, v, err := getEntry(s)
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
		e, v, err := getEntry(s)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(v)
		if err != nil {
			return nil, err
		}
		e.CA = b
	}
	for _, s := range c.RegistryInsecureSkipTlsVerify {
		e, v, err := getEntry(s)
		if err != nil {
			return nil, err
		}
		e.InsecureSkipTlsVerify = utils.ParseBoolOrFalse(v)
	}

	for _, e := range byRegistryAndRepo {
		la.AddEntry(*e)
	}

	return la, nil
}
