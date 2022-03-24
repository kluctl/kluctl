package auth

import (
	"fmt"
	"github.com/kluctl/kluctl/pkg/git/auth/goph"
	"github.com/kluctl/kluctl/pkg/utils"
	"golang.org/x/crypto/ssh"
	"net"
	"os"
	"path/filepath"
	"strconv"
)

func verifyHost(host string, remote net.Addr, key ssh.PublicKey) error {
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
	files := filepath.SplitList(os.Getenv("SSH_KNOWN_HOSTS"))
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

	if !askIsHostTrusted(host, key) {
		return fmt.Errorf("aborted")
	}

	return goph.AddKnownHost(host, remote, key, "")
}

func askIsHostTrusted(host string, key ssh.PublicKey) bool {
	prompt := fmt.Sprintf("Unknown Host: %s\nFingerprint: %s\nWould you like to add it? ", host, ssh.FingerprintSHA256(key))
	return utils.AskForConfirmation(prompt)
}
