package vars

import (
	"github.com/kluctl/go-jinja2"
	"github.com/kluctl/kluctl/v2/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_jinja2"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
)

type VarsCtx struct {
	J2   *jinja2.Jinja2
	Vars *uo.UnstructuredObject
}

func NewVarsCtx(j2 *jinja2.Jinja2) *VarsCtx {
	vc := &VarsCtx{
		J2:   j2,
		Vars: uo.New(),
	}
	return vc
}

func (vc *VarsCtx) Copy() *VarsCtx {
	cp := &VarsCtx{
		J2:   vc.J2,
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

func (vc *VarsCtx) RenderString(t string, searchDirs []string) (string, error) {
	globals, err := vc.Vars.ToMap()
	if err != nil {
		return "", err
	}
	return vc.J2.RenderString(t,
		jinja2.WithSearchDirs(searchDirs),
		jinja2.WithGlobals(globals),
	)
}

func (vc *VarsCtx) RenderStruct(o interface{}) (bool, error) {
	globals, err := vc.Vars.ToMap()
	if err != nil {
		return false, err
	}
	return vc.J2.RenderStruct(o, jinja2.WithGlobals(globals))
}

func (vc *VarsCtx) RenderFile(p string, searchDirs []string) (string, error) {
	globals, err := vc.Vars.ToMap()
	if err != nil {
		return "", err
	}
	ret, err := vc.J2.RenderFile(p,
		jinja2.WithSearchDirs(searchDirs),
		jinja2.WithGlobals(globals),
	)
	if err != nil {
		return "", err
	}

	return ret, nil
}

func (vc *VarsCtx) RenderYamlFile(p string, searchDirs []string, out interface{}) error {
	rendered, err := vc.RenderFile(p, searchDirs)
	if err != nil {
		return err
	}
	err = yaml.ReadYamlString(rendered, out)
	if err != nil {
		return err
	}
	return nil
}

func (vc *VarsCtx) RenderDirectory(sourceDir string, targetDir string, excludePatterns []string, searchDirs []string, templateIgnoreRoot string) error {
	globals, err := vc.Vars.ToMap()
	if err != nil {
		return err
	}
	return vc.J2.RenderDirectory(sourceDir, targetDir, excludePatterns, jinja2.WithGlobals(globals), jinja2.WithSearchDirs(searchDirs), jinja2.WithTemplateIgnoreRootDir(templateIgnoreRoot))
}

func (vc *VarsCtx) CheckConditional(c string) (bool, error) {
	if kluctl_jinja2.IsConditionalTrue(c) {
		return true, nil
	}

	m, err := vc.Vars.ToMap()
	if err != nil {
		return false, err
	}
	c, err = kluctl_jinja2.RenderConditional(vc.J2, m, c)
	if err != nil {
		return false, err
	}
	return kluctl_jinja2.IsConditionalTrue(c), nil
}
