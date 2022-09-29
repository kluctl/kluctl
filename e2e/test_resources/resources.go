package test_resources

import (
	"embed"
	"os"

	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils"
)

//go:embed *.yaml
var Yamls embed.FS

func GetYamlTmpFile(name string) string {
	tmpFile, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}
	tmpFile.Close()

	err = utils.FsCopyFile(Yamls, name, tmpFile.Name())
	if err != nil {
		panic(err)
	}

	return tmpFile.Name()
}

func ApplyYaml(name string, k *test_utils.KindCluster) {
	tmpFile := GetYamlTmpFile(name)
	defer os.Remove(tmpFile)

	_, err := k.Kubectl("apply", "-f", tmpFile)
	if err != nil {
		panic(err)
	}
}

func DeleteYaml(name string, k *test_utils.KindCluster) {
	tmpFile := GetYamlTmpFile(name)
	defer os.Remove(tmpFile)

	_, err := k.Kubectl("delete", "-f", tmpFile)
	if err != nil {
		panic(err)
	}
}
