package deployment

import (
	"fmt"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/pkg/kluctl_project"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
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

	if x, err := dp.CheckWhenTrue(); !x || err != nil {
		return dp, err
	}

	err = dp.loadIncludes()
	if err != nil {
		return nil, fmt.Errorf("failed to load includes for %s: %w", dir, err)
	}

	return dp, nil
}

func (p *DeploymentProject) loadVarsList(varsCtx *vars.VarsCtx, varsList []types.VarsSource) error {
	return p.ctx.VarsLoader.LoadVarsList(p.ctx.Ctx, varsCtx, varsList, p.getRenderSearchDirs(), "")
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
	p.Config.Deployments = append(p.Config.Deployments, types.DeploymentItemConfig{
		Path: utils.Ptr("."),
	})
	return nil
}

func (p *DeploymentProject) processConfig() error {
	for _, item := range p.Config.Deployments {
		if len(item.RenderedObjects) != 0 {
			return fmt.Errorf("renderedObjects is not allowed here")
		}
		if item.RenderedHelmChartConfig != nil {
			return fmt.Errorf("renderedHelmChartConfig is not allowed here")
		}
		if item.RenderedInclude != nil {
			return fmt.Errorf("renderedInclude is not allowed here")
		}
	}

	err := p.loadVarsList(p.VarsCtx, p.Config.Vars)
	if err != nil {
		return fmt.Errorf("failed to load deployment.yml vars: %w", err)
	}

	// If there are no explicit tags set, interpret the path as a tag, which allows to
	// enable/disable single deployments via included/excluded tags
	for i, _ := range p.Config.Deployments {
		item := &p.Config.Deployments[i]
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

func (p *DeploymentProject) CheckWhenTrue() (bool, error) {
	if p.parentProject == nil && p.Config.When != "" {
		return false, fmt.Errorf("the root deployment project can not contain 'when'")
	}

	return p.VarsCtx.CheckConditional(p.Config.When)
}

func (p *DeploymentProject) loadIncludes() error {
	for i, _ := range p.Config.Deployments {
		inc := &p.Config.Deployments[i]
		var err error
		var newProject *DeploymentProject

		whenTrue, err := p.VarsCtx.CheckConditional(inc.When)
		if err != nil {
			return err
		}
		if !whenTrue {
			continue
		}

		if inc.Include != nil {
			newProject, err = p.loadLocalInclude(p.source, filepath.Join(p.relDir, *inc.Include), inc)
			if err != nil {
				return err
			}
		} else if inc.Git != nil {
			ge, err := p.ctx.GitRP.GetEntry(inc.Git.Url.String())
			if err != nil {
				return err
			}
			if inc.Git.Ref != nil && inc.Git.Ref.Ref != "" {
				status.Deprecation(p.ctx.Ctx, "git-include-string-ref", "Passing 'ref' as string into git includes is "+
					"deprecated and support for this will be removed in a future version of Kluctl. Please refer to the "+
					"documentation for details: https://kluctl.io/docs/kluctl/reference/deployments/deployment-yml/#git-includes")
			}
			cloneDir, _, err := ge.GetClonedDir(inc.Git.Ref)
			if err != nil {
				return err
			}
			newProject, err = p.loadLocalInclude(NewSource(cloneDir), inc.Git.SubDir, inc)
			if err != nil {
				return err
			}
		} else if inc.Oci != nil {
			oe, err := p.ctx.OciRP.GetEntry(inc.Oci.Url)
			if err != nil {
				return err
			}
			extractedDir, _, err := oe.GetExtractedDir(inc.Oci.Ref)
			if err != nil {
				return err
			}
			newProject, err = p.loadLocalInclude(NewSource(extractedDir), inc.Oci.SubDir, inc)
			if err != nil {
				return err
			}
		} else {
			continue
		}
		inc.RenderedInclude = &newProject.Config
		newProject.parentProjectInclude = inc
		p.includes[i] = newProject
	}
	return nil
}

func (p *DeploymentProject) loadLocalInclude(source Source, incDir string, inc *types.DeploymentItemConfig) (*DeploymentProject, error) {
	varsCtx := vars.NewVarsCtx(p.VarsCtx.J2)

	libraryFile := yaml.FixPathExt(filepath.Join(source.dir, incDir, ".kluctl-library.yaml"))
	if yaml.Exists(libraryFile) {
		var lib types.KluctlLibraryProject
		err := yaml.ReadYamlFile(libraryFile, &lib)
		if err != nil {
			return nil, err
		}

		if inc.PassVars {
			varsCtx.Vars = p.VarsCtx.Vars.Clone()
			_ = varsCtx.Vars.RemoveNestedField("args") // args should not be merged but taken 1:1
		}

		args := uo.New()
		if inc.Args != nil {
			args = inc.Args.Clone()
		}

		err = kluctl_project.LoadDefaultArgs(lib.Args, args)
		if err != nil {
			return nil, err
		}
		varsCtx.UpdateChild("args", args)
	} else {
		varsCtx = p.VarsCtx.Copy()
	}

	err := p.loadVarsList(varsCtx, inc.Vars)
	if err != nil {
		return nil, err
	}

	newProject, err := NewDeploymentProject(p.ctx, varsCtx, source, incDir, p)
	if err != nil {
		return nil, err
	}

	return newProject, nil
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

func (p *DeploymentProject) GetCommonAnnotations() map[string]string {
	ret := make(map[string]string)
	parents := p.getParents()
	for i, _ := range parents {
		d := parents[len(parents)-i-1]
		uo.MergeStrMap(ret, d.p.Config.CommonAnnotations)
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

func (p *DeploymentProject) getTags() *utils.OrderedMap[string, bool] {
	var tags utils.OrderedMap[string, bool]
	for _, e := range p.getParents() {
		if e.inc != nil {
			tags.SetMultiple(e.inc.Tags, true)
		}
		tags.SetMultiple(e.p.Config.Tags, true)
	}
	return &tags
}

func (p *DeploymentProject) GetIgnoreForDiffs(ignoreTags, ignoreLabels, ignoreAnnotations, ignoreKluctlMetadata bool) []types.IgnoreForDiffItemConfig {
	var ret []types.IgnoreForDiffItemConfig
	for _, e := range p.getParents() {
		ret = append(ret, e.p.Config.IgnoreForDiff...)
	}
	if ignoreTags {
		ret = append(ret, types.IgnoreForDiffItemConfig{FieldPathRegex: []string{`metadata\.labels\["kluctl\.io/tag-.*"\]`}})
	}
	if ignoreLabels {
		ret = append(ret, types.IgnoreForDiffItemConfig{FieldPath: []string{`metadata.labels.*`}})
	}
	if ignoreAnnotations {
		ret = append(ret, types.IgnoreForDiffItemConfig{FieldPath: []string{`metadata.annotations.*`}})
	}
	if ignoreKluctlMetadata {
		ret = append(ret, types.IgnoreForDiffItemConfig{FieldPathRegex: []string{`metadata\.labels\["kluctl\.io/tag-.*"\]`}})
		ret = append(ret, types.IgnoreForDiffItemConfig{FieldPathRegex: []string{`metadata\.labels\["kluctl\.io/discriminator"\]`}})
		ret = append(ret, types.IgnoreForDiffItemConfig{FieldPathRegex: []string{`metadata\.annotations\["kluctl\.io/deployment-item-dir"\]`}})
	}
	return ret
}

func (p *DeploymentProject) GetConflictResolutionConfigs() []types.ConflictResolutionConfig {
	var ret []types.ConflictResolutionConfig
	for _, e := range p.getParents() {
		ret = append(ret, e.p.Config.ConflictResolution...)
	}
	return ret
}
