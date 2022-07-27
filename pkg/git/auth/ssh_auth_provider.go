package auth

import (
	"context"
	"fmt"
	"github.com/kevinburke/ssh_config"
	git_url "github.com/kluctl/kluctl/v2/pkg/git/git-url"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	sshagent "github.com/xanzy/ssh-agent"
	"golang.org/x/crypto/ssh"
	"io"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"sync"
)

type GitSshAuthProvider struct {
	passphrases      map[string][]byte
	passphrasesMutex sync.Mutex
}

type sshDefaultIdentityAndAgent struct {
	ctx          context.Context
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
	cc.HostKeyCallback = buildVerifyHostCallback(a.ctx, nil)
	return cc, nil
}

func (a *sshDefaultIdentityAndAgent) Signers() ([]ssh.Signer, error) {
	return a.signers, nil
}

func (a *sshDefaultIdentityAndAgent) addDefaultIdentity(gitUrl git_url.GitUrl) {
	status.Trace(a.ctx, "trying to add default identity")
	u, err := user.Current()
	if err != nil {
		status.Trace(a.ctx, "No current user: %v", err)
	} else {
		path := filepath.Join(u.HomeDir, ".ssh", "id_rsa")
		signer, err := a.authProvider.readKey(a.ctx, path)
		if err != nil && !os.IsNotExist(err) {
			status.Warning(a.ctx, "Failed to read default identity file for url %s: %v", gitUrl.String(), err)
		} else if signer != nil {
			status.Trace(a.ctx, "...added '%s' as default identity", path)
			a.signers = append(a.signers, signer)
		}
	}
}

func (a *sshDefaultIdentityAndAgent) addConfigIdentities(gitUrl git_url.GitUrl) {
	status.Trace(a.ctx, "trying to add identities from ssh config")
	for _, id := range ssh_config.GetAll(gitUrl.Hostname(), "IdentityFile") {
		expanded := utils.ExpandPath(id)
		status.Trace(a.ctx, "...trying '%s' (expanded: '%s')", id, expanded)
		signer, err := a.authProvider.readKey(a.ctx, expanded)
		if err != nil && !os.IsNotExist(err) {
			status.Warning(a.ctx, "Failed to read key %s for url %s: %v", id, gitUrl.String(), err)
		} else if err == nil {
			status.Trace(a.ctx, "...added '%s' from ssh config", expanded)
			a.signers = append(a.signers, signer)
		}
	}
}

func (a *sshDefaultIdentityAndAgent) addAgentIdentities(gitUrl git_url.GitUrl) {
	status.Trace(a.ctx, "trying to add agent keys")
	agent, _, err := sshagent.New()
	if err != nil {
		status.Warning(a.ctx, "Failed to connect to ssh agent for url %s: %v", gitUrl.String(), err)
	} else {
		signers, err := agent.Signers()
		if err != nil {
			status.Warning(a.ctx, "Failed to get signers from ssh agent for url %s: %v", gitUrl.String(), err)
			return
		}
		status.Trace(a.ctx, "...added %d agent keys", len(signers))
		a.signers = append(a.signers, signers...)
	}
}

func (a *GitSshAuthProvider) BuildAuth(ctx context.Context, gitUrl git_url.GitUrl) AuthMethodAndCA {
	if !gitUrl.IsSsh() {
		return AuthMethodAndCA{}
	}
	if gitUrl.User == nil {
		return AuthMethodAndCA{}
	}

	auth := &sshDefaultIdentityAndAgent{
		ctx:          ctx,
		authProvider: a,
		hostname:     gitUrl.Hostname(),
		user:         gitUrl.User.Username(),
	}

	// Try agent identities first. They might be unencrypted already, making passphrase prompts unnecessary
	auth.addAgentIdentities(gitUrl)
	auth.addDefaultIdentity(gitUrl)
	auth.addConfigIdentities(gitUrl)

	return AuthMethodAndCA{
		AuthMethod: auth,
		PublicKeys: func() []ssh.PublicKey {
			signers, _ := auth.Signers()
			var ret []ssh.PublicKey
			for _, s := range signers {
				ret = append(ret, s.PublicKey())
			}
			return ret
		},
		ClientConfig: auth.ClientConfig,
	}
}

// we defer asking for passphrase so that we don't unnecessarily ask for passphrases for keys that are already provided
// by ssh-agent
type deferredPassphraseKey struct {
	ctx    context.Context
	a      *GitSshAuthProvider
	path   string
	err    error
	parsed ssh.Signer
	mutex  sync.Mutex
}

func (k *deferredPassphraseKey) getPassphrase() ([]byte, error) {
	k.a.passphrasesMutex.Lock()
	defer k.a.passphrasesMutex.Unlock()

	if k.a.passphrases == nil {
		k.a.passphrases = map[string][]byte{}
	}

	passphrase, ok := k.a.passphrases[k.path]
	if ok {
		return passphrase, nil
	}

	passphraseStr, err := status.AskForPassword(k.ctx, fmt.Sprintf("Enter passphrase for key '%s'", k.path))
	if err != nil {
		k.a.passphrases[k.path] = nil
		return nil, err
	}
	passphrase = []byte(passphraseStr)
	k.a.passphrases[k.path] = passphrase
	return passphrase, nil
}

func (k *deferredPassphraseKey) parse() {
	passphrase, err := k.getPassphrase()
	if err != nil {
		k.err = err
		status.Warning(k.ctx, "Failed to parse key %s: %v", k.path, err)
		return
	}

	pemBytes, err := ioutil.ReadFile(k.path)
	if err != nil {
		k.err = err
		status.Warning(k.ctx, "Failed to parse key %s: %v", k.path, err)
		return
	}

	s, err := ssh.ParsePrivateKeyWithPassphrase(pemBytes, passphrase)
	if err != nil {
		k.err = err
		status.Warning(k.ctx, "Failed to parse key %s: %v", k.path, err)
		return
	}
	k.parsed = s
}

type dummyPublicKey struct {
}

func (k *dummyPublicKey) Type() string {
	return "dummy"
}

// Marshal returns the serialized key data in SSH wire format,
// with the name prefix. To unmarshal the returned data, use
// the ParsePublicKey function.
func (k *dummyPublicKey) Marshal() []byte {
	return []byte{}
}

// Verify that sig is a signature on the given data using this
// key. This function will hash the data appropriately first.
func (k *dummyPublicKey) Verify(data []byte, sig *ssh.Signature) error {
	return fmt.Errorf("this is a dummy key")
}

func (k *deferredPassphraseKey) PublicKey() ssh.PublicKey {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if k.parsed == nil && k.err == nil {
		k.parse()
	}
	if k.err != nil {
		return &dummyPublicKey{}
	}
	return k.parsed.PublicKey()
}

func (k *deferredPassphraseKey) Sign(rand io.Reader, data []byte) (*ssh.Signature, error) {
	k.mutex.Lock()
	defer k.mutex.Unlock()

	if k.parsed == nil && k.err == nil {
		k.parse()
	}
	if k.err != nil {
		return nil, k.err
	}
	return k.parsed.Sign(rand, data)
}

func (a *GitSshAuthProvider) readKey(ctx context.Context, path string) (ssh.Signer, error) {
	pemBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	} else {
		signer, err := ssh.ParsePrivateKey(pemBytes)
		if err == nil {
			return signer, nil
		}

		if _, ok := err.(*ssh.PassphraseMissingError); ok {
			signer = &deferredPassphraseKey{
				ctx:  ctx,
				a:    a,
				path: path,
			}
		}

		return signer, nil
	}
}
