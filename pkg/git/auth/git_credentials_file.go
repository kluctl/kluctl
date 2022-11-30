package auth

import (
	"bufio"
	"context"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/git/messages"
	giturls "github.com/whilp/git-urls"
	"os"
	"path/filepath"
)

type GitCredentialsFileAuthProvider struct {
	MessageCallbacks messages.MessageCallbacks
}

func (a *GitCredentialsFileAuthProvider) BuildAuth(ctx context.Context, gitUrl git_url.GitUrl) AuthMethodAndCA {
	if gitUrl.Scheme != "http" && gitUrl.Scheme != "https" {
		return AuthMethodAndCA{}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		a.MessageCallbacks.Warning("Could not determine home directory: %v", err)
		return AuthMethodAndCA{}
	}
	auth := a.tryBuildAuth(gitUrl, filepath.Join(home, ".git-credentials"))
	if auth != nil {
		return *auth
	}

	if xdgHome, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok && xdgHome != "" {
		auth = a.tryBuildAuth(gitUrl, filepath.Join(xdgHome, ".git-credentials"))
		if auth != nil {
			return *auth
		}
	} else {
		auth = a.tryBuildAuth(gitUrl, filepath.Join(home, ".config/.git-credentials"))
		if auth != nil {
			return *auth
		}
	}
	return AuthMethodAndCA{}
}

func (a *GitCredentialsFileAuthProvider) tryBuildAuth(gitUrl git_url.GitUrl, gitCredentialsPath string) *AuthMethodAndCA {
	if !utils.IsFile(gitCredentialsPath) {
		return nil
	}

	f, err := os.Open(gitCredentialsPath)
	if err != nil {
		a.MessageCallbacks.Warning("Failed to open %s: %v", gitCredentialsPath, err)
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
				return &AuthMethodAndCA{
					AuthMethod: &http.BasicAuth{
						Username: u.User.Username(),
						Password: password,
					},
				}
			}
		}
	}
	return nil
}
