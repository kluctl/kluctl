package types

import (
	"encoding/json"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/kluctl/kluctl/lib/yaml"
	"net/url"
	"path"
	"strings"
)

// +kubebuilder:validation:Type=string
type GitUrl struct {
	url.URL `json:"-"`
}

func ParseGitUrl(u string) (*GitUrl, error) {
	// we re-use go-git's parsing capabilities (especially in regard to SCP urls)
	ep, err := transport.NewEndpoint(u)
	if err != nil {
		return nil, err
	}

	if ep.Protocol == "file" {
		return nil, fmt.Errorf("file:// protocol is not supported: %s", u)
	}

	u2 := ep.String()

	// and we also rely on the standard lib to treat escaping properly
	u3, err := url.Parse(u2)
	if err != nil {
		return nil, err
	}

	return &GitUrl{
		URL: *u3,
	}, nil
}

func ParseGitUrlMust(u string) *GitUrl {
	u2, err := ParseGitUrl(u)
	if err != nil {
		panic(err)
	}
	return u2
}

func (in *GitUrl) DeepCopyInto(out *GitUrl) {
	out.URL = in.URL
	if out.URL.User != nil {
		out.URL.User = &*in.URL.User
	}
}

func (u *GitUrl) UnmarshalJSON(b []byte) error {
	var s string
	err := yaml.ReadYamlBytes(b, &s)
	if err != nil {
		return err
	}
	u2, err := ParseGitUrl(s)
	if err != nil {
		return err
	}
	*u = *u2
	return err
}

func (u GitUrl) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}

func (u *GitUrl) IsSsh() bool {
	return u.Scheme == "ssh" || u.Scheme == "git" || u.Scheme == "git+ssh"
}

func (u *GitUrl) NormalizePort() string {
	port := u.Port()
	if port == "" {
		return ""
	}

	defaultPort := ""
	switch u.Scheme {
	case "http":
		defaultPort = "80"
	case "https":
		defaultPort = "443"
	case "git":
		defaultPort = "22"
	case "git+ssh":
		defaultPort = "22"
	case "ssh":
		defaultPort = "22"
	case "ftp":
		defaultPort = "21"
	case "rsync":
		defaultPort = "873"
	case "file":
		break
	default:
		return port
	}

	if defaultPort == "" || port == defaultPort {
		return ""
	}
	return port
}

func (u *GitUrl) Normalize() *GitUrl {
	p := strings.ToLower(u.Path)
	if path.Base(p) != ".git" {
		// we only trim it with it does not end with /.git
		p = strings.TrimSuffix(p, ".git")
	}
	p = strings.TrimSuffix(p, "/")

	hostname := strings.ToLower(u.Hostname())
	port := u.NormalizePort()

	if p != "" && hostname != "" && !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	u2 := *u
	u2.Path = p
	u2.Host = hostname
	if port != "" {
		u2.Host += ":" + port
	}
	return &u2
}

func (u *GitUrl) RepoKey() RepoKey {
	u2 := u.Normalize()
	return NewRepoKey("git", u2.Host, u2.Path)
}
