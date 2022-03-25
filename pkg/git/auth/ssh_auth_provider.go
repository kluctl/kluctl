package auth

import (
	"fmt"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/kevinburke/ssh_config"
	git_url "github.com/kluctl/kluctl/pkg/git/git-url"
	"github.com/kluctl/kluctl/pkg/utils"
	log "github.com/sirupsen/logrus"
	sshagent "github.com/xanzy/ssh-agent"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
)

type GitSshAuthProvider struct {
}

type sshDefaultIdentityAndAgent struct {
	hostname string
	user     string
	signers  []ssh.Signer
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
	return a.signers, nil
}

func (a *sshDefaultIdentityAndAgent) addDefaultIdentity(gitUrl git_url.GitUrl) {
	u, err := user.Current()
	if err != nil {
		log.Debugf("No current user: %v", err)
	} else {
		signer, err := readKey(filepath.Join(u.HomeDir, ".ssh", "id_rsa"))
		if err != nil {
			log.Debugf("Failed to read default identity file for url %s: %v", gitUrl.String(), err)
		} else if signer != nil {
			a.signers = append(a.signers, signer)
		}
	}
}

func (a *sshDefaultIdentityAndAgent) addConfigIdentities(gitUrl git_url.GitUrl) {
	for _, id := range ssh_config.GetAll(gitUrl.Hostname(), "IdentityFile") {
		id = utils.ExpandPath(id)
		signer, err := readKey(id)
		if err != nil && !os.IsNotExist(err) {
			log.Debugf("Failed to read key %s for url %s: %v", id, gitUrl.String(), err)
		} else if err == nil {
			a.signers = append(a.signers, signer)
		}
	}
}

func (a *sshDefaultIdentityAndAgent) addAgentIdentities(gitUrl git_url.GitUrl) {
	agent, _, err := sshagent.New()
	if err != nil {
		log.Debugf("Failed to connect to ssh agent for url %s: %v", gitUrl.String(), err)
	} else {
		signers, err := agent.Signers()
		if err != nil {
			log.Debugf("Failed to get signers from ssh agent for url %s: %v", gitUrl.String(), err)
			return
		}
		a.signers = append(a.signers, signers...)
	}
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
		user:     gitUrl.User.Username(),
	}

	auth.addDefaultIdentity(gitUrl)
	auth.addConfigIdentities(gitUrl)
	auth.addAgentIdentities(gitUrl)

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
