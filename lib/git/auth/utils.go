package auth

import (
	"k8s.io/client-go/util/homedir"
	"strings"
)

func expandHomeDir(p string) string {
	if strings.HasPrefix(p, "~/") {
		p = homedir.HomeDir() + p[1:]
	}
	return p
}
