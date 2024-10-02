package args

import (
	"context"
	"fmt"
	"github.com/gobwas/glob"
	git_auth "github.com/kluctl/kluctl/lib/git/auth"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"os"
	"strings"
)

type GitCredentials struct {
	GitUsername          []string `group:"git" skipenv:"true" help:"Specify username to use for Git basic authentication. Must be in the form --git-username=<host>/<path>=<username>."`
	GitPassword          []string `group:"git" skipenv:"true" help:"Specify password to use for Git basic authentication. Must be in the form --git-password=<host>/<path>=<password>."`
	GitSshKeyFile        []string `group:"git" skipenv:"true" help:"Specify SSH key to use for Git authentication. Must be in the form --git-ssh-key-file=<host>/<path>=<filePath>."`
	GitSshKnownHostsFile []string `group:"git" skipenv:"true" help:"Specify known_hosts file to use for Git authentication. Must be in the form --git-ssh-known-hosts-file=<host>/<path>=<filePath>."`
	GitCAFile            []string `group:"git" skipenv:"true" help:"Specify CA bundle to use for https verification. Must be in the form --git-ca-file=<registry>/<repo>=<filePath>."`
}

func (c *GitCredentials) BuildAuthProvider(ctx context.Context) (git_auth.GitAuthProvider, error) {
	la := &git_auth.ListAuthProvider{}
	if c == nil {
		return la, nil
	}

	var byHostPath utils.OrderedMap[string, *git_auth.AuthEntry]

	getEntry := func(s string, expectValue bool) (*git_auth.AuthEntry, string, error) {
		x := strings.SplitN(s, "=", 2)
		if expectValue && len(x) != 2 {
			return nil, "", fmt.Errorf("expected value")
		}
		k := x[0]
		e, ok := byHostPath.Get(k)
		if !ok {
			x := strings.SplitN(k, "/", 2)
			e = &git_auth.AuthEntry{}
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

	for _, s := range c.GitUsername {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		e.Username = v
	}
	for _, s := range c.GitPassword {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		e.Password = v
	}
	for _, s := range c.GitSshKeyFile {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(v)
		if err != nil {
			return nil, err
		}
		e.SshKey = b
	}
	for _, s := range c.GitSshKnownHostsFile {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(v)
		if err != nil {
			return nil, err
		}
		e.KnownHosts = b
	}
	for _, s := range c.GitCAFile {
		e, v, err := getEntry(s, true)
		if err != nil {
			return nil, err
		}
		b, err := os.ReadFile(v)
		if err != nil {
			return nil, err
		}
		e.CABundle = b
	}

	for _, e := range byHostPath.ListValues() {
		la.AddEntry(*e)
	}

	return la, nil
}
