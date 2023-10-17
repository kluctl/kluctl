package auth

import (
	"bufio"
	"context"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"github.com/kluctl/kluctl/v2/pkg/git/messages"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"net/url"
	"os"
	"path/filepath"
)

type GitCredentialsFileAuthProvider struct {
	MessageCallbacks messages.MessageCallbacks
}

func (a *GitCredentialsFileAuthProvider) BuildAuth(ctx context.Context, gitUrl types.GitUrl) (AuthMethodAndCA, error) {
	if gitUrl.Scheme != "http" && gitUrl.Scheme != "https" {
		return AuthMethodAndCA{}, nil
	}
	a.MessageCallbacks.Trace("GitCredentialsFileAuthProvider: BuildAuth for %s", gitUrl.String())

	home, err := os.UserHomeDir()
	if err != nil {
		a.MessageCallbacks.Warning("Could not determine home directory: %v", err)
		return AuthMethodAndCA{}, err
	}
	auth := a.tryBuildAuth(gitUrl, filepath.Join(home, ".git-credentials"))
	if auth != nil {
		return *auth, nil
	}

	if xdgHome, ok := os.LookupEnv("XDG_CONFIG_HOME"); ok && xdgHome != "" {
		auth = a.tryBuildAuth(gitUrl, filepath.Join(xdgHome, ".git-credentials"))
		if auth != nil {
			return *auth, nil
		}
	} else {
		auth = a.tryBuildAuth(gitUrl, filepath.Join(home, ".config/.git-credentials"))
		if auth != nil {
			return *auth, nil
		}
	}
	return AuthMethodAndCA{}, nil
}

func (a *GitCredentialsFileAuthProvider) tryBuildAuth(gitUrl types.GitUrl, gitCredentialsPath string) *AuthMethodAndCA {
	st, err := os.Stat(gitCredentialsPath)
	if err != nil || st.Mode().IsDir() {
		return nil
	}

	a.MessageCallbacks.Trace("GitCredentialsFileAuthProvider: trying file %s", gitCredentialsPath)

	f, err := os.Open(gitCredentialsPath)
	if err != nil {
		a.MessageCallbacks.Warning("Failed to open %s: %v", gitCredentialsPath, err)
		return nil
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	for s.Scan() {
		if s.Text() == "" || s.Text()[0] == '#' {
			continue
		}
		u, err := types.ParseGitUrl(s.Text())
		if err != nil {
			continue
		}
		// create temporary url without password, which can be printed
		tmpU := *u
		tmpU.User = url.User(u.User.Username())

		if u.User == nil || u.User.Username() == "" {
			a.MessageCallbacks.Trace("GitCredentialsFileAuthProvider: ignoring %s", tmpU.String())
			continue
		}
		a.MessageCallbacks.Trace("GitCredentialsFileAuthProvider: trying %s", tmpU.String())

		if u.Host != gitUrl.Host {
			a.MessageCallbacks.Trace("GitCredentialsFileAuthProvider: host does not match")
			continue
		}
		if gitUrl.User != nil && gitUrl.User.Username() != u.User.Username() {
			a.MessageCallbacks.Trace("GitCredentialsFileAuthProvider: user does not match")
			continue
		}
		password, ok := u.User.Password()
		if !ok {
			a.MessageCallbacks.Trace("GitCredentialsFileAuthProvider: no password provided")
			continue
		}

		a.MessageCallbacks.Trace("GitCredentialsFileAuthProvider: matched")
		return &AuthMethodAndCA{
			AuthMethod: &http.BasicAuth{
				Username: u.User.Username(),
				Password: password,
			},
		}
	}
	return nil
}
