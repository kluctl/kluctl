package ssh_pool

import (
	"fmt"
	"github.com/fluxcd/go-git/v5/plumbing/transport/ssh"
	"strconv"
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
	if ssh.DefaultSSHConfig == nil {
		return
	}

	configHost := ssh.DefaultSSHConfig.Get(host, "Hostname")
	if configHost != "" {
		host = configHost
		found = true
	}

	if !found {
		return
	}

	configPort := ssh.DefaultSSHConfig.Get(host, "Port")
	if configPort != "" {
		if i, err := strconv.Atoi(configPort); err == nil {
			port = i
		}
	}

	addr = fmt.Sprintf("%s:%d", host, port)
	return
}
