package git_url

import (
	"fmt"
	giturls "github.com/whilp/git-urls"
	"net/url"
	"strings"
)

type GitUrl struct {
	url.URL
}

func Parse(u string) (*GitUrl, error) {
	u2, err := giturls.Parse(u)
	if err != nil {
		return nil, err
	}
	return &GitUrl{*u2}, nil
}

func (u *GitUrl) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	err := unmarshal(&s)
	if err != nil {
		return err
	}
	u2, err := Parse(s)
	if err != nil {
		return err
	}
	*u = *u2
	return err
}

func (u GitUrl) MarshalYAML() (interface{}, error) {
	return u.String(), nil
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
	if strings.HasSuffix(path, ".git") {
		path = path[:len(path)-len(".git")]
	}
	if strings.HasSuffix(path, "/") {
		path = path[:len(path)-1]
	}
	hostname := strings.ToLower(u.Hostname())
	port := u.NormalizePort()

	u2 := *u
	u2.Path = path
	u2.Host = hostname
	if port != "" {
		u2.Host += ":" + port
	}
	return &u2
}

func (u *GitUrl) NormalizedRepoKey() string {
	u2 := u.Normalize()
	return fmt.Sprintf("%s:%s", u2.Host, u2.Path)
}
