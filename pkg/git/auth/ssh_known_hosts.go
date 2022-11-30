package auth

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/git/auth/goph"
	"github.com/kluctl/kluctl/v2/pkg/git/messages"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"golang.org/x/crypto/ssh"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

var askHostMutex sync.Mutex

func buildVerifyHostCallback(messageCallbacks messages.MessageCallbacks, knownHosts []byte) func(hostname string, remote net.Addr, key ssh.PublicKey) error {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		return verifyHost(messageCallbacks, hostname, remote, key, knownHosts)
	}
}

func verifyHost(messageCallbacks messages.MessageCallbacks, host string, remote net.Addr, key ssh.PublicKey, knownHosts []byte) error {
	// Ensure only one prompt happens at a time
	askHostMutex.Lock()
	defer askHostMutex.Unlock()

	hostKeyChecking := true
	if x, ok := os.LookupEnv("KLUCTL_SSH_DISABLE_STRICT_HOST_KEY_CHECKING"); ok {
		if b, err := strconv.ParseBool(x); err == nil && b {
			hostKeyChecking = false
		}
	}
	if !hostKeyChecking {
		return nil
	}

	allowAdd := false
	var files []string
	if knownHosts == nil {
		files = filepath.SplitList(os.Getenv("SSH_KNOWN_HOSTS"))
		if len(files) == 0 {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}

			f := filepath.Join(home, ".ssh", "known_hosts")
			if !utils.Exists(filepath.Dir(f)) {
				err = os.MkdirAll(filepath.Dir(f), 0o700)
				if err != nil {
					return err
				}
			}

			files = append(files, f)
			allowAdd = true
		}
	} else {
		tmpFile, err := os.CreateTemp(utils.GetTmpBaseDir(), "known_hosts-")
		if err != nil {
			return err
		}
		defer func() {
			_ = tmpFile.Close()
			_ = os.Remove(tmpFile.Name())
		}()
		_, err = tmpFile.Write(knownHosts)
		if err != nil {
			return err
		}
		files = append(files, tmpFile.Name())
	}

	for _, f := range files {
		hostFound, err := goph.CheckKnownHost(host, remote, key, f)
		if hostFound && err != nil {
			return err
		}
		if hostFound && err == nil {
			return nil
		}
	}
	if !allowAdd {
		return fmt.Errorf("host not found and SSH_KNOWN_HOSTS has been set")
	}

	if !askIsHostTrusted(messageCallbacks, host, key) {
		return fmt.Errorf("aborted")
	}

	return goph.AddKnownHost(host, remote, key, "")
}

func askIsHostTrusted(messageCallbacks messages.MessageCallbacks, host string, key ssh.PublicKey) bool {
	prompt := fmt.Sprintf("Unknown Host: %s\nFingerprint: %s\nWould you like to add it? ", host, ssh.FingerprintSHA256(key))
	return messageCallbacks.AskForConfirmation(prompt)
}
