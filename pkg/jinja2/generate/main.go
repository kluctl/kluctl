package main

import (
	"github.com/kluctl/kluctl/v2/pkg/utils/embed_util/packer"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func main() {
	pipWheel()
}

func pipWheel() {
	_ = os.RemoveAll("python_src/wheel")
	_ = os.MkdirAll("python_src/wheel", 0o700)

	cmd := exec.Command("pip3", "wheel", "-r", "../requirements.txt")
	cmd.Dir = "python_src/wheel"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		panic(err)
	}

	wheels, err := os.ReadDir("python_src/wheel")
	if err != nil {
		panic(err)
	}
	for _, w := range wheels {
		if !strings.HasSuffix(w.Name(), ".whl") {
			continue
		}

		cmd = exec.Command("unzip", w.Name())
		cmd.Dir = "python_src/wheel"

		err = cmd.Run()
		if err != nil {
			panic(err)
		}

		err = os.Remove(filepath.Join("python_src/wheel", w.Name()))
		if err != nil {
			panic(err)
		}
	}

	err = packer.Pack("embed/python_src.tar.gz", "python_src", "*")
	if err != nil {
		panic(err)
	}
}
