package types

import (
	"encoding/json"
	"fmt"
	"github.com/kluctl/kluctl/lib/yaml"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

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

type ProjectKey struct {
	RepoKey RepoKey `json:"repoKey,omitempty"`
	SubDir  string  `json:"subDir,omitempty"`
}

func (k ProjectKey) Less(o ProjectKey) bool {
	if k.RepoKey != o.RepoKey {
		return k.RepoKey.String() < o.RepoKey.String()
	}
	if k.SubDir != o.SubDir {
		return k.SubDir < o.SubDir
	}
	return false
}
