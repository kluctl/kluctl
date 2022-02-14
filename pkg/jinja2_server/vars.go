package jinja2_server

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/codablock/kluctl/pkg/yaml"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type VarsCtx struct {
	JS   *Jinja2Server
	Vars *uo.UnstructuredObject
}

func NewVarsCtx(js *Jinja2Server) *VarsCtx {
	vc := &VarsCtx{
		JS:   js,
		Vars: uo.New(),
	}
	return vc
}

func (vc *VarsCtx) Copy() *VarsCtx {
	cp := &VarsCtx{
		JS:   vc.JS,
		Vars: vc.Vars.Clone(),
	}
	return cp
}

func (vc *VarsCtx) Update(vars *uo.UnstructuredObject) {
	vc.Vars.Merge(vars)
}

func (vc *VarsCtx) UpdateChild(child string, vars *uo.UnstructuredObject) {
	vc.Vars.MergeChild(child, vars)
}

func (vc *VarsCtx) UpdateChildFromStruct(child string, o interface{}) error {
	other, err := uo.FromStruct(o)
	if err != nil {
		return err
	}
	vc.UpdateChild(child, other)
	return nil
}

func (vc *VarsCtx) LoadVarsList(k *k8s.K8sCluster, searchDirs []string, varsList []*types.VarsListItem) error {
	for _, v := range varsList {
		if v.Values != nil {
			vc.Update(v.Values)
		} else if v.File != nil {
			err := vc.loadVarsFile(*v.File, searchDirs)
			if err != nil {
				return err
			}
		} else if v.ClusterConfigMap != nil {
			ref := types.NewObjectRef("", "v1", "ConfigMap", v.ClusterConfigMap.Name, v.ClusterConfigMap.Namespace)
			err := vc.loadVarsFromK8sObject(k, ref, v.ClusterConfigMap.Key)
			if err != nil {
				return err
			}
		} else if v.ClusterSecret != nil {
			ref := types.NewObjectRef("", "v1", "Secret", v.ClusterSecret.Name, v.ClusterSecret.Namespace)
			err := vc.loadVarsFromK8sObject(k, ref, v.ClusterSecret.Key)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("invalid vars entry: %v", v)
		}
	}

	return nil
}

func (vc *VarsCtx) loadVarsFile(p string, searchDirs []string) error {
	var newVars uo.UnstructuredObject
	err := vc.RenderYamlFile(p, searchDirs, &newVars)
	if err != nil {
		return fmt.Errorf("failed to load vars from %s: %w", p, err)
	}
	vc.Update(&newVars)
	return nil
}

func (vc *VarsCtx) loadVarsFromK8sObject(k *k8s.K8sCluster, ref types.ObjectRef, key string) error {
	o, _, err := k.GetSingleObject(ref)
	if err != nil {
		return err
	}

	value, found, err := unstructured.NestedString(o.UnstructuredContent(), "data", key)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("key %s not found in %s on cluster %s", key, ref.String(), k.Context())
	}

	err = vc.loadVarsFromString(value)
	if err != nil {
		return fmt.Errorf("failed to load vars from kubernetes object %s and key %s: %w", ref.String(), key, err)
	}
	return nil
}

func (vc *VarsCtx) loadVarsFromString(s string) error {
	var newVars uo.UnstructuredObject
	err := vc.renderYamlString(s, &newVars)
	if err != nil {
		return err
	}
	vc.Update(&newVars)
	return nil
}

func (vc *VarsCtx) renderYamlString(s string, out interface{}) error {
	ret, err := vc.JS.RenderString(s, nil, vc.Vars)
	if err != nil {
		return err
	}

	err = yaml.ReadYamlString(ret, out)
	if err != nil {
		return err
	}

	return nil
}

func (vc *VarsCtx) RenderYamlFile(p string, searchDirs []string, out interface{}) error {
	ret, err := vc.JS.RenderFile(p, searchDirs, vc.Vars)
	if err != nil {
		return err
	}

	err = yaml.ReadYamlString(ret, out)
	if err != nil {
		return err
	}

	return nil
}

func (vc *VarsCtx) RenderDirectory(rootDir string, searchDirs []string, relSourceDir string, excludePatterns []string, subdir string, targetDir string) error {
	return vc.JS.RenderDirectory(rootDir, searchDirs, relSourceDir, excludePatterns, subdir, targetDir, vc.Vars)
}
