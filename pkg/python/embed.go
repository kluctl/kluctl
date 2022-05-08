package python

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/embed_util"
	"path/filepath"
	"runtime"
)

//go:generate go run ./generate

var embeddedPythonPath string

func init() {
	embeddedPythonPath = decompressPython()
}

func decompressPython() string {
	tarName := fmt.Sprintf("embed/python-%s-%s.tar.gz", runtime.GOOS, runtime.GOARCH)
	tgz, err := pythonLib.Open(tarName)
	if err != nil {
		panic(err)
	}
	defer tgz.Close()

	fileList, err := pythonLib.Open(tarName + ".files")
	if err != nil {
		panic(err)
	}
	defer fileList.Close()

	path := filepath.Join(utils.GetTmpBaseDir(), fmt.Sprintf("python-%s", runtime.GOOS))
	path, err = embed_util.ExtractTarToTmp(tgz, fileList, path)
	if err != nil {
		panic(err)
	}

	return path
}
