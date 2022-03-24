package auth

import (
	"bufio"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	git_url "github.com/kluctl/kluctl/pkg/git/git-url"
	"github.com/kluctl/kluctl/pkg/utils"
	log "github.com/sirupsen/logrus"
	giturls "github.com/whilp/git-urls"
	"os"
	"path/filepath"
)

type GitCredentialsFileAuthProvider struct {
}

func (a *GitCredentialsFileAuthProvider) BuildAuth(gitUrl git_url.GitUrl) transport.AuthMethod {
	if gitUrl.Scheme != "http" && gitUrl.Scheme != "https" {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		log.Warningf("Could not determine home directory: %v", err)
		return nil
	}
	auth := a.tryBuildAuth(gitUrl, filepath.Join(home, ".git-credentials"))
	if auth != nil {
		return auth
	}

	if xdgHome, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok && xdgHome != "" {
		auth = a.tryBuildAuth(gitUrl, filepath.Join(xdgHome, ".git-credentials"))
		if auth != nil {
			return auth
		}
	} else {
		auth = a.tryBuildAuth(gitUrl, filepath.Join(home, ".config/.git-credentials"))
		if auth != nil {
			return auth
		}
	}
	return nil
}

func (a *GitCredentialsFileAuthProvider) tryBuildAuth(gitUrl git_url.GitUrl, gitCredentialsPath string) transport.AuthMethod {
	if !utils.IsFile(gitCredentialsPath) {
		return nil
	}

	f, err := os.Open(gitCredentialsPath)
	if err != nil {
		log.Warningf("Failed to open %s: %v", gitCredentialsPath, err)
		return nil
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		u, err := giturls.Parse(s.Text())
		if err != nil {
			continue
		}
		if u.Host != gitUrl.Host {
			continue
		}
		if u.User == nil {
			continue
		}
		if password, ok := u.User.Password(); ok {
			if u.User.Username() != "" {
				return &http.BasicAuth{
					Username: u.User.Username(),
					Password: password,
				}
			}
		}
	}
	return nil
}
