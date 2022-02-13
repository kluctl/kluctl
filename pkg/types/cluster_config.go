package types

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"path"
)

type ClusterConfig2 struct {
	Name    string                 `yaml:"name" validate:"required"`
	Context string                 `yaml:"context" validate:"required"`
	Vars    map[string]interface{} `yaml:"vars,omitempty"`
}

type ClusterConfig struct {
	Cluster *ClusterConfig2 `yaml:"cluster"`
}

// TODO remove custom unmarshaller when https://github.com/goccy/go-yaml/pull/220 gets released
func (cc *ClusterConfig2) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var m map[string]interface{}
	err := unmarshal(&m)
	if err != nil {
		return err
	}

	nameI, ok := m["name"]
	if !ok {
		return fmt.Errorf("name is missing in cluster config")
	}
	contextI, ok := m["context"]
	if !ok {
		return fmt.Errorf("context is missing in cluster config")
	}

	cc.Name, ok = nameI.(string)
	if !ok {
		return fmt.Errorf("name is not a string")
	}
	cc.Context, ok = contextI.(string)
	if !ok {
		return fmt.Errorf("context is not a string")
	}

	delete(m, "name")
	delete(m, "context")
	cc.Vars = m
	return nil
}

func (cc *ClusterConfig2) MarshalYAML() (interface{}, error) {
	m := map[string]interface{}{
		"name": cc.Name,
		"context": cc.Context,
	}
	utils.MergeObject(m, cc.Vars)
	return m, nil
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
