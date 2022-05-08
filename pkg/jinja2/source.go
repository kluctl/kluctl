package jinja2

import (
	"embed"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/embed_util"
	"path/filepath"
)

//go:generate go run ./generate

//go:embed embed/python_src.*
var pythonSrc embed.FS
var pythonSrcExtracted string

func init() {
	srcDir, err := extractSource()
	if err != nil {
		panic(err)
	}
	pythonSrcExtracted = srcDir
}

func extractSource() (string, error) {
	tgz, err := pythonSrc.Open("embed/python_src.tar.gz")
	if err != nil {
		return "", err
	}
	defer tgz.Close()

	fileList, err := pythonSrc.Open("embed/python_src.tar.gz.files")
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
