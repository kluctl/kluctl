package types

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/codablock/kluctl/pkg/yaml"
	"path/filepath"
)

type ClusterConfig2 struct {
	Name    string                 `yaml:"name" validate:"required"`
	Context string                 `yaml:"context" validate:"required"`
	Vars    *uo.UnstructuredObject `yaml:"vars,omitempty"`
}

type ClusterConfig struct {
	Cluster *ClusterConfig2 `yaml:"cluster"`
}

// TODO remove custom unmarshaller when https://github.com/goccy/go-yaml/pull/220 gets released
func (cc *ClusterConfig2) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var u uo.UnstructuredObject
	err := unmarshal(&u)
	if err != nil {
		return err
	}

	name, ok, err := u.GetNestedString("name")
	if !ok {
		return fmt.Errorf("name is missing (or not a string) in cluster config")
	}
	context, ok, err := u.GetNestedString("context")
	if !ok {
		return fmt.Errorf("context is missing (or not a string) in cluster config")
	}

	cc.Name = name
	cc.Context = context

	_ = u.RemoveNestedField("name")
	_ = u.RemoveNestedField("context")

	cc.Vars = &u
	return nil
}

func (cc *ClusterConfig2) MarshalYAML() (interface{}, error) {
	o := uo.FromMap(map[string]interface{}{
		"name":    cc.Name,
		"context": cc.Context,
	})
	o.Merge(cc.Vars)
	return o, nil
}

func LoadClusterConfig(clusterDir string, clusterName string) (*ClusterConfig, error) {
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name must be specified")
	}

	p := yaml.FixPathExt(filepath.Join(clusterDir, fmt.Sprintf("%s.yml", clusterName)))
	if !utils.IsFile(p) {
		return nil, fmt.Errorf("cluster config for %s not found", clusterName)
	}

	var config ClusterConfig
	err := yaml.ReadYamlFile(p, &config)
	if err != nil {
		return nil, err
	}

	if config.Cluster.Name != clusterName {
		return nil, fmt.Errorf("cluster name in config (%s) does not match requested cluster name %s", config.Cluster.Name, clusterName)
	}

	return &config, nil
}
