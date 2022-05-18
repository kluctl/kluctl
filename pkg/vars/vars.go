package vars

import (
	"github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/jinja2"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
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
