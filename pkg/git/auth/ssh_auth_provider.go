package auth

import (
	"fmt"
	git_url "github.com/codablock/kluctl/pkg/git/git-url"
	"github.com/go-git/go-git/v5/plumbing/transport"
	git_ssh "github.com/go-git/go-git/v5/plumbing/transport/ssh"
	log "github.com/sirupsen/logrus"
	sshagent "github.com/xanzy/ssh-agent"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"io/ioutil"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

type GitSshAuthProvider struct {
}

type sshDefaultIdentityAndAgent struct {
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
	cc.HostKeyCallback = getHostKeyCallback()
	return cc, nil
}

func getHostKeyCallback() ssh.HostKeyCallback {
	hostKeyChecking := true
	if x, ok := os.LookupEnv("KLUCTL_SSH_DISABLE_STRICT_HOST_KEY_CHECKING"); ok {
		if b, err := strconv.ParseBool(x); err == nil && b {
			hostKeyChecking = false
		}
	}
	if hostKeyChecking {
		cb, err := git_ssh.NewKnownHostsCallback()
		if err != nil {
			return nil
		}
		return cb
	} else {
		return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		}
	}
}

func (a *sshDefaultIdentityAndAgent) Signers() ([]ssh.Signer, error) {
	var ret []ssh.Signer
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
		user: gitUrl.User.Username(),
	}

	u, err := user.Current()
	if err != nil {
		log.Debugf("No current user: %v", err)
	} else {
		pemBytes, err := ioutil.ReadFile(filepath.Join(u.HomeDir, ".ssh", "id_rsa"))
		if err != nil {
			log.Debugf("Failed to read default identity file for url %s: %v", gitUrl.String(), err)
		} else {
			signer, err := ssh.ParsePrivateKey(pemBytes)
			if err != nil {
				log.Debugf("Failed to parse default identity for url %s: %v", gitUrl.String(), err)
			}
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
