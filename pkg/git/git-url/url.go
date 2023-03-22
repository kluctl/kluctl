package git_url

import (
	"encoding/json"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/git/git-url/giturls"
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

func (u *GitUrl) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
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
	username := ""
	if u.User != nil && u.User.Username() != "" {
		username = u.User.Username() + "@"
	}
	return fmt.Sprintf("%s%s:%s%s", username, u2.Hostname(), u2.Port(), u2.Path)
}
