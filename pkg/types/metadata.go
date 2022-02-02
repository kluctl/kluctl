package types

import (
	"github.com/codablock/kluctl/pkg/utils"
)

type InvolvedRepo struct {
	RefPattern string            `yaml:"refPattern"`
	Refs       map[string]string `yaml:"refs"`
}

type ArchiveMetadata struct {
	InvolvedRepos map[string][]InvolvedRepo `yaml:"involvedRepo"`
	Targets       []*Target                 `yaml:"targets"`
}

func LoadArchiveMetadata(p string) (*ArchiveMetadata, error) {
	o := &ArchiveMetadata{}
	return o, utils.ReadYamlFile(p, o)
}
