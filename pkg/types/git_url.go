package types

import (
	"encoding/json"
	"fmt"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"net/url"
	"path"
	"regexp"
	"strconv"
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

// +kubebuilder:validation:Type=string
type RepoKey struct {
	Type string `json:"-"`
	Host string `json:"-"`
	Path string `json:"-"`
}

func NewRepoKey(type_ string, host string, path string) RepoKey {
	path = strings.TrimSuffix(path, "/")
	path = strings.TrimPrefix(path, "/")
	return RepoKey{
		Type: type_,
		Host: host,
		Path: path,
	}
}

func NewRepoKeyFromUrl(urlIn string) (RepoKey, error) {
	u, err := url.Parse(urlIn)
	if err != nil {
		return RepoKey{}, err
	}
	t := "git"
	if u.Scheme == "oci" {
		t = "oci"
	}
	p := u.Path
	if t == "git" {
		p = strings.TrimSuffix(p, ".git")
	}
	return NewRepoKey(t, u.Host, p), nil
}

func NewRepoKeyFromGitUrl(urlIn string) (RepoKey, error) {
	u, err := ParseGitUrl(urlIn)
	if err != nil {
		return RepoKey{}, err
	}
	return NewRepoKeyFromUrl(u.String())
}

var hostNameRegex = regexp.MustCompile(`^(([a-zA-Z]|[a-zA-Z][a-zA-Z0-9\-]*[a-zA-Z0-9])\.)*([A-Za-z]|[A-Za-z][A-Za-z0-9\-]*[A-Za-z0-9])$`)
var ipRegex = regexp.MustCompile(`^((25[0-5]|(2[0-4]|1\d|[1-9]|)\d)\.?\b){4}$`)

func ParseRepoKey(s string, defaultType string) (RepoKey, error) {
	if s == "" {
		return RepoKey{}, nil
	}

	var t string
	if strings.HasPrefix(s, "git://") {
		t = "git"
		s = strings.TrimPrefix(s, "git://")
	} else if strings.HasPrefix(s, "oci://") {
		t = "oci"
		s = strings.TrimPrefix(s, "oci://")
	} else {
		t = defaultType
	}

	s2 := strings.SplitN(s, "/", 2)
	if len(s2) != 2 {
		return RepoKey{}, fmt.Errorf("invalid repo key: %s", s)
	}

	var host, port string
	if strings.Contains(s2[0], ":") {
		s3 := strings.SplitN(s2[0], ":", 2)
		host = s3[0]
		port = s3[1]
	} else {
		host = s2[0]
	}

	if !hostNameRegex.MatchString(host) && !ipRegex.MatchString(host) {
		return RepoKey{}, fmt.Errorf("invalid repo key: %s", s)
	}

	if port != "" {
		if _, err := strconv.ParseInt(port, 10, 32); err != nil {
			return RepoKey{}, fmt.Errorf("invalid repo key: %s", s)
		}
	}

	return RepoKey{
		Type: t,
		Host: s2[0],
		Path: s2[1],
	}, nil
}

func (u RepoKey) String() string {
	if u.Host == "" && u.Path == "" {
		return ""
	}
	t := u.Type
	if t == "" {
		t = "git"
	}
	return fmt.Sprintf("%s://%s/%s", t, u.Host, u.Path)
}

func (u *RepoKey) UnmarshalJSON(b []byte) error {
	var s string
	err := yaml.ReadYamlBytes(b, &s)
	if err != nil {
		return err
	}
	if s == "" {
		u.Type = ""
		u.Host = ""
		u.Path = ""
		return nil
	}
	x, err := ParseRepoKey(s, "git")
	if err != nil {
		return err
	}
	*u = x
	return nil
}

func (u RepoKey) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(u.String())
	if err != nil {
		return nil, err
	}

	return b, err
}
