package python

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func PythonCmd(args []string) *exec.Cmd {
	var exePath string
	if runtime.GOOS == "windows" {
		exePath = filepath.Join(embeddedPythonPath, "python.exe")
	} else {
		exePath = filepath.Join(embeddedPythonPath, "bin/python3")
	}

	cmd := exec.Command(exePath, args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("PYTHONHOME=%s", embeddedPythonPath))

	return cmd
}
