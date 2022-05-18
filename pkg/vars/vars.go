package vars

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/jinja2"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/types/k8s"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io/ioutil"
	"os"
)

type VarsCtx struct {
	J2   *jinja2.Jinja2
	grc  *git.MirroredGitRepoCollection
	Vars *uo.UnstructuredObject
}

func NewVarsCtx(j2 *jinja2.Jinja2, grc *git.MirroredGitRepoCollection) *VarsCtx {
	vc := &VarsCtx{
		J2:   j2,
		grc:  grc,
		Vars: uo.New(),
	}
	return vc
}

func (vc *VarsCtx) Copy() *VarsCtx {
	cp := &VarsCtx{
		J2:   vc.J2,
		grc:  vc.grc,
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
		} else if v.Git != nil {
			err := vc.loadVarsGitFile(v.Git)
			if err != nil {
				return err
			}
		} else if v.ClusterConfigMap != nil {
			ref := k8s2.NewObjectRef("", "v1", "ConfigMap", v.ClusterConfigMap.Name, v.ClusterConfigMap.Namespace)
			err := vc.loadVarsFromK8sObject(k, ref, v.ClusterConfigMap.Key)
			if err != nil {
				return err
			}
		} else if v.ClusterSecret != nil {
			ref := k8s2.NewObjectRef("", "v1", "Secret", v.ClusterSecret.Name, v.ClusterSecret.Namespace)
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

func (vc *VarsCtx) loadVarsGitFile(gitFile *types.VarsListItemGit) error {
	mr, err := vc.grc.GetMirroredGitRepo(gitFile.Url, true, true, true)
	if err != nil {
		return fmt.Errorf("failed to load vars from git repository %s: %w", gitFile.Url.String(), err)
	}

	tmpDir, err := ioutil.TempDir(utils.GetTmpBaseDir(), "git-vars")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	file, err := mr.ReadFile(gitFile.Ref, gitFile.Path)
	if err != nil {
		return fmt.Errorf("failed to load vars from git repository %s and path %s: %w", gitFile.Url.String(), gitFile.Path, err)
	}

	return vc.loadVarsFromString(string(file))
}

func (vc *VarsCtx) loadVarsFromK8sObject(k *k8s.K8sCluster, ref k8s2.ObjectRef, key string) error {
	if k == nil {
		return nil
	}

	o, _, err := k.GetSingleObject(ref)
	if err != nil {
		return err
	}

	value, found, err := o.GetNestedString("data", key)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("key %s not found in %s on cluster", key, ref.String())
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
	ret, err := vc.J2.RenderString(s, nil, vc.Vars)
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
	ret, err := vc.J2.RenderFile(p, searchDirs, vc.Vars)
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
	return vc.J2.RenderDirectory(rootDir, searchDirs, relSourceDir, excludePatterns, subdir, targetDir, vc.Vars)
}
