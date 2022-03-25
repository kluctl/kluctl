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
	"sync"
)

type GitSshAuthProvider struct {
	passphrases map[string][]byte
	passphrasesMutex sync.Mutex
}

type sshDefaultIdentityAndAgent struct {
	authProvider *GitSshAuthProvider
	hostname     string
	user         string
	signers      []ssh.Signer
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
		signer, err := a.authProvider.readKey(filepath.Join(u.HomeDir, ".ssh", "id_rsa"))
		if err != nil && !os.IsNotExist(err) {
			log.Warningf("Failed to read default identity file for url %s: %v", gitUrl.String(), err)
		} else if signer != nil {
			a.signers = append(a.signers, signer)
		}
	}
}

func (a *sshDefaultIdentityAndAgent) addConfigIdentities(gitUrl git_url.GitUrl) {
	for _, id := range ssh_config.GetAll(gitUrl.Hostname(), "IdentityFile") {
		id = utils.ExpandPath(id)
		signer, err := a.authProvider.readKey(id)
		if err != nil && !os.IsNotExist(err) {
			log.Warningf("Failed to read key %s for url %s: %v", id, gitUrl.String(), err)
		} else if err == nil {
			a.signers = append(a.signers, signer)
		}
	}
}

func (a *sshDefaultIdentityAndAgent) addAgentIdentities(gitUrl git_url.GitUrl) {
	agent, _, err := sshagent.New()
	if err != nil {
		log.Warningf("Failed to connect to ssh agent for url %s: %v", gitUrl.String(), err)
	} else {
		signers, err := agent.Signers()
		if err != nil {
			log.Warningf("Failed to get signers from ssh agent for url %s: %v", gitUrl.String(), err)
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
		authProvider: a,
		hostname: gitUrl.Hostname(),
		user:     gitUrl.User.Username(),
	}

	auth.addDefaultIdentity(gitUrl)
	auth.addConfigIdentities(gitUrl)
	auth.addAgentIdentities(gitUrl)

	return auth
}

func (a *GitSshAuthProvider) readKey(path string) (ssh.Signer, error) {
	pemBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	} else {
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err == nil {
			return signer, nil
		}

		if _, ok := err.(*ssh.PassphraseMissingError); ok {
			passphrase := a.getPassphrase(path)
			if passphrase == nil {
				return nil, err
			}
			return ssh.ParsePrivateKeyWithPassphrase(pemBytes, passphrase)
		}

		return signer, nil
	}
}

func (a *GitSshAuthProvider) getPassphrase(path string) []byte {
	a.passphrasesMutex.Lock()
	defer a.passphrasesMutex.Unlock()

	if a.passphrases == nil {
		a.passphrases = map[string][]byte{}
	}

	passphrase, ok := a.passphrases[path]
	if ok {
		return passphrase
	}

	passphraseStr, err := utils.AskForPassword(fmt.Sprintf("Enter passphrase for key '%s'", path))
	if err != nil {
		log.Warning(err)
		a.passphrases[path] = nil
		return nil
	}
	a.passphrases[path] = passphrase
	return []byte(passphraseStr)
}
