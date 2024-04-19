package main

import (
	"github.com/kluctl/go-embed-python/embed_util"
)

func main() {
	err := embed_util.BuildAndWriteFilesList("../install/controller")
	if err != nil {
		panic(err)
	}
}
