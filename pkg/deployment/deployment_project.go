package deployment

import (
	"fmt"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_jinja2"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"path/filepath"
	"strings"
)

type DeploymentProject struct {
	ctx SharedContext

	VarsCtx *vars.VarsCtx

	source Source
	relDir string
	absDir string

	Config types.DeploymentProjectConfig

	includes map[int]*DeploymentProject

	parentProject        *DeploymentProject
	parentProjectInclude *types.DeploymentItemConfig
}

func NewDeploymentProject(ctx SharedContext, varsCtx *vars.VarsCtx, source Source, relDir string, parentProject *DeploymentProject) (*DeploymentProject, error) {
	dp := &DeploymentProject{
		ctx:           ctx,
		VarsCtx:       varsCtx.Copy(),
		source:        source,
		relDir:        relDir,
		parentProject: parentProject,
		includes:      map[int]*DeploymentProject{},
	}

	dir, err := securejoin.SecureJoin(dp.source.dir, dp.relDir)
	if err != nil {
		return nil, err
	}

	if !utils.IsDirectory(dir) {
		return nil, fmt.Errorf("%s does not exist or is not a directory", dir)
	}

	dp.absDir = dir

	err = dp.loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load deployment config for %s: %w", dir, err)
	}

	if !kluctl_jinja2.IsConditionalTrue(dp.Config.When) {
		return dp, nil
	}

	err = dp.loadIncludes()
	if err != nil {
		return nil, fmt.Errorf("failed to load includes for %s: %w", dir, err)
	}

	if dp.Config.SealedSecrets != nil {
		status.Deprecation(ctx.Ctx, "sealed-secrets-config", "'sealedSecrets' is deprecated in deployment projects. Support for it will be removed in a future kluctl release.")
	}

	return dp, nil
}

func (p *DeploymentProject) loadVarsList(varsCtx *vars.VarsCtx, varsList []*types.VarsSource) error {
	return p.ctx.VarsLoader.LoadVarsList(varsCtx, varsList, p.getRenderSearchDirs(), "")
}

func (p *DeploymentProject) loadConfig() error {
	configPath := filepath.Join(p.absDir, "deployment.yml")
	if !yaml.Exists(configPath) {
		if p.parentProject == nil {
			// the root project is allowed to not have a deployment.yaml
			// in that case, it is treated as a simple deployment with a single deployment item pointing to "."
			return p.generateSingleKustomizeProject()
		} else if yaml.Exists(filepath.Join(p.absDir, "kustomization.yml")) {
			return fmt.Errorf("deployment.yml not found but folder %s contains a kustomization.yml", p.absDir)
		} else {
			return fmt.Errorf("%s not found", configPath)
		}
	}
	configPath = yaml.FixPathExt(configPath)

	err := p.VarsCtx.RenderYamlFile(configPath, p.getRenderSearchDirs(), &p.Config)
	if err != nil {
		return fmt.Errorf("failed to load deployment.yml: %w", err)
	}

	return p.processConfig()
}

func (p *DeploymentProject) generateSingleKustomizeProject() error {
	p.Config.Deployments = append(p.Config.Deployments, &types.DeploymentItemConfig{
		Path: utils.StrPtr("."),
	})
	return nil
}

func (p *DeploymentProject) processConfig() error {
	err := p.loadVarsList(p.VarsCtx, p.Config.Vars)
	if err != nil {
		return fmt.Errorf("failed to load deployment.yml vars: %w", err)
	}

	// If there are no explicit tags set, interpret the path as a tag, which allows to
	// enable/disable single deployments via included/excluded tags
	for _, item := range p.Config.Deployments {
		if len(item.Tags) != 0 {
			continue
		}
		if item.Path != nil {
			item.Tags = []string{filepath.Base(*item.Path)}
		} else if item.Include != nil {
			item.Tags = []string{filepath.Base(*item.Include)}
		}
	}

	err = p.checkDeploymentDirs()
	if err != nil {
		return err
	}

	err = p.renderConditionals()
	if err != nil {
		return err
	}

	return nil
}

func (p *DeploymentProject) checkDeploymentDirs() error {
	for _, di := range p.Config.Deployments {
		if di.Path == nil {
			continue
		}

		diDir, err := securejoin.SecureJoin(p.source.dir, filepath.Join(p.relDir, *di.Path))
		if err != nil {
			return err
		}

		if !strings.HasPrefix(diDir, p.source.dir) {
			return fmt.Errorf("path/include is not part of the deployment project: %s", *di.Path)
		}

		if !utils.Exists(diDir) {
			return fmt.Errorf("deployment directory does not exist: %s", *di.Path)
		}
		if !utils.IsDirectory(diDir) {
			return fmt.Errorf("deployment path is not a directory: %s", *di.Path)
		}
	}
	return nil
}

func (p *DeploymentProject) renderConditionals() error {
	if p.parentProject == nil && p.Config.When != "" {
		return fmt.Errorf("the root deployment project can not contain 'when'")
	}

	vars, err := p.VarsCtx.Vars.ToMap()
	if err != nil {
		return err
	}
	r, err := kluctl_jinja2.RenderConditional(p.VarsCtx.J2, vars, p.Config.When)
	if err != nil {
		return err
	}
	p.Config.When = r

	if !kluctl_jinja2.IsConditionalTrue(p.Config.When) {
		// No need to render individual deployment item conditionals
		return nil
	}

	conditionals := make([]string, 0, len(p.Config.Deployments))
	for _, di := range p.Config.Deployments {
		conditionals = append(conditionals, di.When)
	}
	rendered, err := kluctl_jinja2.RenderConditionals(p.VarsCtx.J2, vars, conditionals)
	if err != nil {
		return err
	}
	for i, r := range rendered {
		p.Config.Deployments[i].When = r
	}
	return nil
}

func (p *DeploymentProject) loadIncludes() error {
	for i, inc := range p.Config.Deployments {
		var err error
		var newProject *DeploymentProject

		if !kluctl_jinja2.IsConditionalTrue(inc.When) {
			continue
		}

		if inc.Include != nil {
			newProject, err = p.loadLocalInclude(p.source, filepath.Join(p.relDir, *inc.Include), inc.Vars)
			if err != nil {
				return err
			}
		} else if inc.Git != nil {
			ge, err := p.ctx.RP.GetEntry(inc.Git.Url)
			if err != nil {
				return err
			}
			cloneDir, _, err := ge.GetClonedDir(inc.Git.Ref)
			if err != nil {
				return err
			}
			newProject, err = p.loadLocalInclude(NewSource(cloneDir), inc.Git.SubDir, inc.Vars)
			if err != nil {
				return err
			}
		} else {
			continue
		}
		newProject.parentProjectInclude = inc
		p.includes[i] = newProject
	}
	return nil
}

func (p *DeploymentProject) loadLocalInclude(source Source, incDir string, vars []*types.VarsSource) (*DeploymentProject, error) {
	varsCtx := p.VarsCtx.Copy()
	err := p.loadVarsList(varsCtx, vars)
	if err != nil {
		return nil, err
	}

	newProject, err := NewDeploymentProject(p.ctx, varsCtx, source, incDir, p)
	if err != nil {
		return nil, err
	}

	return newProject, nil
}

func (p *DeploymentProject) getRenderedOutputPattern() string {
	for _, x := range p.getParents() {
		if x.p.Config.SealedSecrets != nil && x.p.Config.SealedSecrets.OutputPattern != nil {
			return *x.p.Config.SealedSecrets.OutputPattern
		}
	}
	return p.ctx.DefaultSealedSecretsOutputPattern
}

func (p *DeploymentProject) getRootProject() *DeploymentProject {
	if p.parentProject == nil {
		return p
	}
	return p.parentProject.getRootProject()
}

type deploymentProjectAndIncludeItem struct {
	p   *DeploymentProject
	inc *types.DeploymentItemConfig
}

func (p *DeploymentProject) getParents() []deploymentProjectAndIncludeItem {
	var parents []deploymentProjectAndIncludeItem
	var inc *types.DeploymentItemConfig
	d := p
	for d != nil {
		parents = append(parents, deploymentProjectAndIncludeItem{p: d, inc: inc})
		inc = d.parentProjectInclude
		d = d.parentProject
	}
	return parents
}

func (p *DeploymentProject) getChildren(recursive bool, includeSelf bool) []*DeploymentProject {
	var children []*DeploymentProject
	if includeSelf {
		children = append(children, p)
	}
	for _, d := range p.includes {
		children = append(children, d)
		if recursive {
			children = append(children, d.getChildren(true, false)...)
		}
	}
	return children
}

func (p *DeploymentProject) getRenderSearchDirs() []string {
	var ret []string
	for _, d := range p.getParents() {
		if d.p.source != p.source {
			// only allow searching inside same source project
			continue
		}
		ret = append(ret, d.p.absDir)
	}
	return ret
}

func (p *DeploymentProject) GetCommonLabels() map[string]string {
	ret := make(map[string]string)
	parents := p.getParents()
	for i, _ := range parents {
		d := parents[len(parents)-i-1]
		uo.MergeStrMap(ret, d.p.Config.CommonLabels)
	}
	return ret
}

func (p *DeploymentProject) getOverrideNamespace() *string {
	for _, e := range p.getParents() {
		if e.p.Config.OverrideNamespace != nil {
			return e.p.Config.OverrideNamespace
		}
	}
	return nil
}

func (p *DeploymentProject) getTags() *utils.OrderedMap {
	var tags utils.OrderedMap
	for _, e := range p.getParents() {
		if e.inc != nil {
			tags.SetMultiple(e.inc.Tags, true)
		}
		tags.SetMultiple(e.p.Config.Tags, true)
	}
	return &tags
}

func (p *DeploymentProject) GetIgnoreForDiffs(ignoreTags, ignoreLabels, ignoreAnnotations bool) []*types.IgnoreForDiffItemConfig {
	var ret []*types.IgnoreForDiffItemConfig
	for _, e := range p.getParents() {
		ret = append(ret, e.p.Config.IgnoreForDiff...)
	}
	if ignoreTags {
		ret = append(ret, &types.IgnoreForDiffItemConfig{FieldPath: []string{`metadata.labels."kluctl.io/tag-*"`}})
	}
	if ignoreLabels {
		ret = append(ret, &types.IgnoreForDiffItemConfig{FieldPath: []string{`metadata.labels.*`}})
	}
	if ignoreAnnotations {
		ret = append(ret, &types.IgnoreForDiffItemConfig{FieldPath: []string{`metadata.annotations.*`}})
	}
	return ret
}
