package webui

import (
	"embed"
	log "github.com/sirupsen/logrus"
	"io/fs"
)

//go:embed ui/build
var uiBuildFS embed.FS
var uiFS fs.FS

func init() {
	var err error
	uiFS, err = fs.Sub(uiBuildFS, "ui/build")
	if err != nil {
		log.Fatal("failed to get ui fs", err)
	}
}
