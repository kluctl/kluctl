package auth

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/kevinburke/ssh_config"
	"github.com/kluctl/kluctl/v2/lib/git/messages"
	"github.com/kluctl/kluctl/v2/pkg/types"
	sshagent "github.com/xanzy/ssh-agent"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sync"
)

type GitSshAuthProvider struct {
	MessageCallbacks messages.MessageCallbacks

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
	cc.HostKeyCallback = buildVerifyHostCallback(a.authProvider.MessageCallbacks, nil)
	return cc, nil
}

func (a *sshDefaultIdentityAndAgent) Signers() ([]ssh.Signer, error) {
	return a.signers, nil
}

func (a *sshDefaultIdentityAndAgent) addDefaultIdentities(gitUrl types.GitUrl) {
	a.authProvider.MessageCallbacks.Trace("trying to add default identity")
	u, err := user.Current()
	if err != nil {
		a.authProvider.MessageCallbacks.Trace("No current user: %v", err)
		return
	}

	doAdd := func(name string) {
		path := filepath.Join(u.HomeDir, ".ssh", name)
		signer, err := a.authProvider.readKey(a.ctx, path)
		if err != nil && !os.IsNotExist(err) {
			a.authProvider.MessageCallbacks.Warning("Failed to read default identity file for url %s: %v", gitUrl.String(), err)
		} else if signer != nil {
			a.authProvider.MessageCallbacks.Trace("...added '%s' as default identity", path)
			a.signers = append(a.signers, signer)
		}
	}

	doAdd("id_rsa")
	doAdd("id_ecdsa")
	doAdd("id_ed25519")
	doAdd("id_xmss")
	doAdd("id_dsa")
}

func (a *sshDefaultIdentityAndAgent) addConfigIdentities(gitUrl types.GitUrl) {
	a.authProvider.MessageCallbacks.Trace("trying to add identities from ssh config")
	for _, id := range ssh_config.GetAll(gitUrl.Hostname(), "IdentityFile") {
		expanded := expandHomeDir(id)
		a.authProvider.MessageCallbacks.Trace("...trying '%s' (expanded: '%s')", id, expanded)
		signer, err := a.authProvider.readKey(a.ctx, expanded)
		if err != nil && !os.IsNotExist(err) {
			a.authProvider.MessageCallbacks.Warning("Failed to read key %s for url %s: %v", id, gitUrl.String(), err)
		} else if err == nil {
			a.authProvider.MessageCallbacks.Trace("...added '%s' from ssh config", expanded)
			a.signers = append(a.signers, signer)
		}
	}
}

func (a *sshDefaultIdentityAndAgent) createAgent(gitUrl types.GitUrl) (agent.Agent, error) {
	if runtime.GOOS == "windows" {
		a, _, err := sshagent.New()
		return a, err
	}

	// see `man ssh_config`

	sshAuthSock := ssh_config.Get(gitUrl.Hostname(), "IdentityAgent")
	if sshAuthSock == "none" {
		return nil, nil
	}
	if sshAuthSock == "" || sshAuthSock == "SSH_AUTH_SOCK" {
		a, _, err := sshagent.New()
		return a, err
	}

	sshAuthSock = os.ExpandEnv(sshAuthSock)
	sshAuthSock = expandHomeDir(sshAuthSock)

	conn, err := net.Dial("unix", sshAuthSock)
	if err != nil {
		return nil, fmt.Errorf("error connecting to unix socket: %v", err)
	}

	return agent.NewClient(conn), nil
}

func (a *sshDefaultIdentityAndAgent) addAgentIdentities(gitUrl types.GitUrl) {
	a.authProvider.MessageCallbacks.Trace("trying to add agent keys")
	agent, err := a.createAgent(gitUrl)
	if err != nil {
		a.authProvider.MessageCallbacks.Warning("Failed to connect to ssh agent for url %s: %v", gitUrl.String(), err)
	} else {
		if agent == nil {
			return
		}
		signers, err := agent.Signers()
		if err != nil {
			a.authProvider.MessageCallbacks.Warning("Failed to get signers from ssh agent for url %s: %v", gitUrl.String(), err)
			return
		}
		a.authProvider.MessageCallbacks.Trace("...added %d agent keys", len(signers))
		a.signers = append(a.signers, signers...)
	}
}

func (a *GitSshAuthProvider) BuildAuth(ctx context.Context, gitUrl types.GitUrl) (AuthMethodAndCA, error) {
	if !gitUrl.IsSsh() {
		return AuthMethodAndCA{}, nil
	}
	if gitUrl.User == nil {
		return AuthMethodAndCA{}, nil
	}

	auth := &sshDefaultIdentityAndAgent{
		ctx:          ctx,
		authProvider: a,
		hostname:     gitUrl.Hostname(),
		user:         gitUrl.User.Username(),
	}

	// Try agent identities first. They might be unencrypted already, making passphrase prompts unnecessary
	auth.addAgentIdentities(gitUrl)
	auth.addDefaultIdentities(gitUrl)
	auth.addConfigIdentities(gitUrl)

	return AuthMethodAndCA{
		AuthMethod: auth,
		Hash: func() ([]byte, error) {
			signers, err := auth.Signers()
			if err != nil {
				return nil, err
			}
			return buildHashForList(signers)
		},
	}, nil
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

	passphraseStr, err := k.a.MessageCallbacks.AskForPassword(fmt.Sprintf("Enter passphrase for key '%s'", k.path))
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
		k.a.MessageCallbacks.Warning("Failed to parse key %s: %v", k.path, err)
		return
	}

	pemBytes, err := os.ReadFile(k.path)
	if err != nil {
		k.err = err
		k.a.MessageCallbacks.Warning("Failed to parse key %s: %v", k.path, err)
		return
	}

	s, err := ssh.ParsePrivateKeyWithPassphrase(pemBytes, passphrase)
	if err != nil {
		k.err = err
		k.a.MessageCallbacks.Warning("Failed to parse key %s: %v", k.path, err)
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

func (k *deferredPassphraseKey) Hash() ([]byte, error) {
	pemBytes, err := os.ReadFile(k.path)
	if err != nil {
		return nil, err
	}

	h := sha256.New()
	_ = binary.Write(h, binary.LittleEndian, "dpk")
	_ = binary.Write(h, binary.LittleEndian, pemBytes)

	return h.Sum(nil), nil
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
	pemBytes, err := os.ReadFile(path)
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
