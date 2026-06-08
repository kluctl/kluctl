package ssh_pool

import (
	"fmt"
	"strconv"

	"github.com/go-git/go-git/v6/plumbing/transport/ssh"
	"github.com/kevinburke/ssh_config"
)

func getHostWithPort(host string, port int) string {
	if addr, found := doGetHostWithPortFromSSHConfig(host, port); found {
		return addr
	}

	if port <= 0 {
		port = ssh.DefaultPort
	}

	return fmt.Sprintf("%s:%d", host, port)
}

func doGetHostWithPortFromSSHConfig(host string, port int) (addr string, found bool) {
	if ssh_config.DefaultUserSettings == nil {
		return
	}

	configHost := ssh_config.DefaultUserSettings.Get(host, "Hostname")
	if configHost != "" {
		host = configHost
		found = true
	}

	if !found {
		return
	}

	configPort := ssh_config.DefaultUserSettings.Get(host, "Port")
	if configPort != "" {
		if i, err := strconv.Atoi(configPort); err == nil {
			port = i
		}
	}

	addr = fmt.Sprintf("%s:%d", host, port)
	return
}
