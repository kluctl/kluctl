package auth

import (
	"fmt"
	git_url "github.com/codablock/kluctl/pkg/git/git-url"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/go-git/go-git/v5/plumbing/transport"
	git_ssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	log "github.com/sirupsen/logrus"
	sshagent "github.com/xanzy/ssh-agent"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"io/ioutil"
	"os/user"
	"path/filepath"
)

type GitSshAuthProvider struct {
}

type sshDefaultIdentityAndAgent struct {
	hostname        string
	user            string
	defaultIdentity ssh.Signer
	agent           agent.Agent
}

func (a *sshDefaultIdentityAndAgent) String() string {
	return fmt.Sprintf("user: %s, name: %s", a.user, a.Name())
}

func (a *sshDefaultIdentityAndAgent) Name() string {
	return "ssh-default-identity-and-agent"
}

func (a *sshDefaultIdentityAndAgent) ClientConfig() (*ssh.ClientConfig, error) {
	cc := &ssh.ClientConfig{
		User: a.user,
		Auth: []ssh.AuthMethod{ssh.PublicKeysCallback(a.Signers)},
	}
	cc.HostKeyCallback = verifyHost
	return cc, nil
}

func (a *sshDefaultIdentityAndAgent) Signers() ([]ssh.Signer, error) {
	var ret []ssh.Signer
	identityFromConfig := git_ssh.DefaultSSHConfig.Get(a.hostname, "IdentityFile")
	if identityFromConfig != "" {
		identityFromConfig = utils.ExpandPath(identityFromConfig)
		signer, err := readKey(identityFromConfig)
		if err != nil {
			return nil, err
		}
		ret = append(ret, signer)
		return ret, nil
	}
	if a.defaultIdentity != nil {
		ret = append(ret, a.defaultIdentity)
	}
	if a.agent != nil {
		s, err := a.agent.Signers()
		if err != nil {
			return nil, err
		}
		ret = append(ret, s...)
	}
	return ret, nil
}

func (a *GitSshAuthProvider) BuildAuth(gitUrl git_url.GitUrl) transport.AuthMethod {
	if !gitUrl.IsSsh() {
		return nil
	}
	if gitUrl.User == nil {
		return nil
	}

	auth := &sshDefaultIdentityAndAgent{
		hostname: gitUrl.Hostname(),
		user: gitUrl.User.Username(),
	}

	u, err := user.Current()
	if err != nil {
		log.Debugf("No current user: %v", err)
	} else {
		signer, err := readKey(filepath.Join(u.HomeDir, ".ssh", "id_rsa"))
		if err != nil {
			log.Debugf("Failed to read default identity file for url %s: %v", gitUrl.String(), err)
		} else if signer != nil {
			auth.defaultIdentity = signer
		}
	}

	agent, _, err := sshagent.New()
	if err != nil {
		log.Debugf("Failed to connect to ssh agent for url %s: %v", gitUrl.String(), err)
	} else {
		auth.agent = agent
	}

	return auth
}

func readKey(path string) (ssh.Signer, error) {
	pemBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	} else {
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err != nil {
			return nil, err
		}
		return signer, nil
	}
}