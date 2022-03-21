package jinja2

import (
	"embed"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/embed_util"
	log "github.com/sirupsen/logrus"
	"path/filepath"
)

//go:embed python_src.tar.gz python_src.tar.gz.files
var pythonSrc embed.FS
var pythonSrcExtracted string

func init() {
	srcDir, err := extractSource()
	if err != nil {
		log.Panic(err)
	}
	pythonSrcExtracted = srcDir
}

func extractSource() (string, error) {
	tgz, err := pythonSrc.Open("python_src.tar.gz")
	if err != nil {
		return "", err
	}
	defer tgz.Close()

	fileList, err := pythonSrc.Open("python_src.tar.gz.files")
	if err != nil {
		return "", err
	}
	defer fileList.Close()

	libPath := filepath.Join(utils.GetTmpBaseDir(), "jinja2-src")
	libPath, err = embed_util.ExtractTarToTmp(tgz, fileList, libPath)
	if err != nil {
		return "", err
	}

	return libPath, nil
}
