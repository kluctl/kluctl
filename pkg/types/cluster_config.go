package types

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"path"
)

type ClusterConfig2 struct {
	Name    string                 `yaml:"name" validate:"required"`
	Context string                 `yaml:"context" validate:"required"`
	Vars    map[string]interface{} `yaml:",inline"`
}

type ClusterConfig struct {
	Cluster ClusterConfig2 `yaml:"cluster"`
}

func LoadClusterConfig(clusterDir string, clusterName string) (*ClusterConfig, error) {
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name must be specified")
	}

	p := path.Join(clusterDir, fmt.Sprintf("%s.yml", clusterName))
	if !utils.IsFile(p) {
		return nil, fmt.Errorf("cluster config for %s not found", clusterName)
	}

	var config ClusterConfig
	err := utils.ReadYamlFile(p, &config)
	if err != nil {
		return nil, err
	}

	if config.Cluster.Name != clusterName {
		return nil, fmt.Errorf("cluster name in config (%s) does not match requested cluster name %s", config.Cluster.Name, clusterName)
	}

	return &config, nil
}
