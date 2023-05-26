package types

import (
	"encoding/json"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/git/giturls"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"net/url"
	"strings"
)

// +kubebuilder:validation:Type=string
type GitUrl struct {
	url.URL `json:"-"`
}

func ParseGitUrl(u string) (*GitUrl, error) {
	// we explicitly only test ParseTransport and ParseScp to avoid parsing local Git urls (as done by giturls.Parse)
	u2, err := giturls.ParseTransport(u)
	if err == nil {
		return &GitUrl{*u2}, nil
	}
	u2, err = giturls.ParseScp(u)
	if err == nil {
		return &GitUrl{*u2}, nil
	}
	return nil, fmt.Errorf("failed to parse git url: %s", u)
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
	path := strings.ToLower(u.Path)
	path = strings.TrimSuffix(path, ".git")
	path = strings.TrimSuffix(path, "/")

	hostname := strings.ToLower(u.Hostname())
	port := u.NormalizePort()

	if path != "" && hostname != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	u2 := *u
	u2.Path = path
	u2.Host = hostname
	if port != "" {
		u2.Host += ":" + port
	}
	return &u2
}

func (u *GitUrl) RepoKey() GitRepoKey {
	u2 := u.Normalize()
	path := strings.TrimPrefix(u2.Path, "/")
	return GitRepoKey{
		Host: u2.Host,
		Path: path,
	}
}

// +kubebuilder:validation:Type=string
type GitRepoKey struct {
	Host string `json:"-"`
	Path string `json:"-"`
}

func ParseGitRepoKey(s string) (GitRepoKey, error) {
	if s == "" {
		return GitRepoKey{}, nil
	}

	s2 := strings.SplitN(s, "/", 2)
	if len(s2) != 2 {
		return GitRepoKey{}, fmt.Errorf("invalid git repo key: %s", s)
	}
	return GitRepoKey{
		Host: s2[0],
		Path: s2[1],
	}, nil
}

func (u GitRepoKey) String() string {
	if u.Host == "" && u.Path == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s", u.Host, u.Path)
}

func (u *GitRepoKey) UnmarshalJSON(b []byte) error {
	var s string
	err := yaml.ReadYamlBytes(b, &s)
	if err != nil {
		return err
	}
	if s == "" {
		u.Host = ""
		u.Path = ""
		return nil
	}
	x, err := ParseGitRepoKey(s)
	if err != nil {
		return err
	}
	*u = x
	return nil
}

func (u GitRepoKey) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(u.String())
	if err != nil {
		return nil, err
	}

	return b, err
}
