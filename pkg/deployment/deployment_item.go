package deployment

import (
	"fmt"
	"github.com/hashicorp/go-multierror"
	"github.com/kluctl/kluctl/lib/yaml"
	"github.com/kluctl/kluctl/v2/pkg/helm"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/sops"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/kustomize"
	securefs "github.com/kluctl/kluctl/v2/pkg/utils/flux_utils/kustomize/filesys"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type DeploymentItem struct {
	ctx SharedContext

	Project   *DeploymentProject
	Inclusion *utils.Inclusion
	Config    *types.DeploymentItemConfig
	VarsCtx   *vars.VarsCtx
	dir       *string
	index     int

	// These values come from the metadata of the kustomization.yml
	Barrier       bool
	WaitReadiness bool

	Objects []*uo.UnstructuredObject
	Tags    *utils.OrderedMap[string, bool]

	RenderedSourceRootDir string
	RelToSourceItemDir    string
	RelToProjectItemDir   string
	RelRenderedDir        string
	RenderedDir           string
	renderedYamlPath      string
}

func NewDeploymentItem(ctx SharedContext, project *DeploymentProject, collection *DeploymentCollection, config *types.DeploymentItemConfig, dir *string, index int) (*DeploymentItem, error) {
	di := &DeploymentItem{
		ctx:       ctx,
		Project:   project,
		Inclusion: collection.Inclusion,
		Config:    config,
		VarsCtx:   project.VarsCtx.Copy(),
		dir:       dir,
		index:     index,
	}

	var err error

	// collect tags
	di.Tags = di.Project.getTags()
	di.Tags.SetMultiple(di.Config.Tags, true)

	if di.dir != nil {
		di.RelToSourceItemDir, err = filepath.Rel(di.Project.source.dir, *di.dir)
		if err != nil {
			return nil, err
		}

		di.RelToProjectItemDir, err = filepath.Rel(di.Project.relDir, di.RelToSourceItemDir)
		if err != nil {
			return nil, err
		}

		di.RelRenderedDir = di.RelToSourceItemDir
		if di.index != 0 {
			di.RelRenderedDir = fmt.Sprintf("%s-%d", di.RelRenderedDir, di.index)
		}

		di.RenderedSourceRootDir = filepath.Join(collection.ctx.RenderDir, di.Project.source.id)
		di.RenderedDir = filepath.Join(di.RenderedSourceRootDir, di.RelRenderedDir)
		di.renderedYamlPath = filepath.Join(di.RenderedDir, ".rendered.yml")
	}

	err = di.Project.loadVarsList(di.VarsCtx, di.Config.Vars)
	if err != nil {
		return nil, err
	}

	return di, nil
}

func (di *DeploymentItem) getCommonLabels() map[string]string {
	l := di.Project.GetCommonLabels()
	if di.ctx.Discriminator != "" {
		l["kluctl.io/discriminator"] = di.ctx.Discriminator
	}
	i := 0
	for _, t := range di.Tags.ListKeys() {
		l[fmt.Sprintf("kluctl.io/tag-%d", i)] = t
		i += 1
	}
	return l
}

func (di *DeploymentItem) getCommonAnnotations() map[string]string {
	a := di.Project.GetCommonAnnotations()
	a["kluctl.io/deployment-item-dir"] = filepath.ToSlash(di.RelToSourceItemDir)
	if di.Config.SkipDeleteIfTags {
		a["kluctl.io/skip-delete-if-tags"] = "true"
	}
	return a
}

func (di *DeploymentItem) render() error {
	if di.dir == nil {
		return nil
	}

	err := os.MkdirAll(di.RenderedDir, 0o700)
	if err != nil {
		return err
	}

	var excludePatterns []string
	excludePatterns = append(excludePatterns, "**/.git")

	err = filepath.WalkDir(*di.dir, func(p string, d fs.DirEntry, err error) error {
		if d == nil {
			return nil
		}
		if d.IsDir() {
			relDir, err := filepath.Rel(*di.dir, p)
			if err != nil {
				return err
			}
			if yaml.Exists(filepath.Join(p, "Chart.yaml")) {
				// never try to render helm charts
				ep := fmt.Sprintf("%s/**", relDir)
				ep = filepath.ToSlash(ep)
				excludePatterns = append(excludePatterns, ep)
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	searchDirs := di.Project.getRenderSearchDirs()
	// also add deployment item dir to search dirs
	searchDirs = append([]string{*di.dir}, searchDirs...)

	return di.VarsCtx.RenderDirectory(
		filepath.Join(di.Project.source.dir, di.RelToSourceItemDir),
		di.RenderedDir,
		excludePatterns,
		searchDirs,
		di.Project.source.dir,
	)
}

func (di *DeploymentItem) isHelmChartYaml(p string) bool {
	_, file := filepath.Split(p)
	file = strings.ToLower(file)
	return file == "helm-chart.yml" || file == "helm-chart.yaml"
}

func (di *DeploymentItem) isHelmValuesYaml(p string) bool {
	_, file := filepath.Split(p)
	file = strings.ToLower(file)
	return file == "helm-values.yml" || file == "helm-values.yaml"
}

func (di *DeploymentItem) newHelmRelease(subDir string) (*helm.Release, error) {
	configPath := yaml.FixPathExt(filepath.Join(di.RenderedDir, subDir, "helm-chart.yaml"))

	var helmChartsDir string
	relDir := di.RelToSourceItemDir
	for {
		p := filepath.Join(di.Project.source.dir, relDir, ".helm-charts")
		if utils.IsDirectory(p) {
			helmChartsDir = p
			break
		}
		if relDir == "." {
			break
		}
		relDir = filepath.Dir(relDir)
	}

	hr, err := helm.NewRelease(di.ctx.Ctx, di.Project.source.dir, filepath.Join(di.RelToSourceItemDir, subDir), configPath, helmChartsDir, di.ctx.HelmAuthProvider, di.ctx.OciAuthProvider)
	if err != nil {
		return nil, err
	}
	return hr, nil
}

func (di *DeploymentItem) renderHelmCharts() error {
	if di.dir == nil {
		return nil
	}

	err := filepath.Walk(di.RenderedDir, func(p string, info fs.FileInfo, err error) error {
		if !di.isHelmChartYaml(p) {
			return nil
		}

		subDir, err := filepath.Rel(di.RenderedDir, filepath.Dir(p))
		if err != nil {
			return err
		}

		hr, err := di.newHelmRelease(subDir)
		if err != nil {
			return err
		}

		ky, err := di.readKustomizationYaml(subDir)
		if err == nil && ky != nil {
			resources, _, _ := ky.GetNestedStringList("resources")
			found := false
			for _, r := range resources {
				if r == hr.GetOutputPath() {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("%s/kustomization.yaml does not include the rendered helm chart: %s", di.RelRenderedDir, hr.GetOutputPath())
			}
		}

		di.Config.RenderedHelmChartConfig = hr.Config

		return hr.Render(di.ctx.Ctx, di.ctx.K, di.ctx.K8sVersion, di.ctx.SopsDecrypter)
	})
	if err != nil {
		return err
	}
	return nil
}

func (di *DeploymentItem) buildInclusionEntries() []utils.InclusionEntry {
	var values []utils.InclusionEntry
	for _, t := range di.Tags.ListKeys() {
		values = append(values, utils.InclusionEntry{Type: "tag", Value: t})
	}
	if di.dir != nil {
		dir := filepath.ToSlash(di.RelToSourceItemDir)
		values = append(values, utils.InclusionEntry{Type: "deploymentItemDir", Value: dir})
	}
	return values
}

func (di *DeploymentItem) CheckInclusionForDeploy() bool {
	if di.Inclusion == nil {
		return true
	}
	if di.Config.OnlyRender {
		return true
	}
	if di.Config.AlwaysDeploy {
		return true
	}
	if di.Config.Barrier {
		return true
	}
	values := di.buildInclusionEntries()
	return di.Inclusion.CheckIncluded(values, false)
}

func (di *DeploymentItem) readKustomizationYaml(subDir string) (*uo.UnstructuredObject, error) {
	if di.dir == nil {
		return nil, nil
	}

	kustomizeYamlPath := yaml.FixPathExt(filepath.Join(di.RenderedDir, subDir, "kustomization.yml"))
	if !utils.IsFile(kustomizeYamlPath) {
		return nil, nil
	}

	ky, err := uo.FromFile(kustomizeYamlPath)
	if err != nil {
		return nil, err
	}

	return ky, err
}

func (di *DeploymentItem) generateKustomizationYaml(subDir string) (*uo.UnstructuredObject, error) {
	kustomizeYamlPath := yaml.FixPathExt(filepath.Join(di.RenderedDir, subDir, "kustomization.yml"))
	if utils.IsFile(kustomizeYamlPath) {
		return nil, nil
	}

	des, err := os.ReadDir(filepath.Join(di.RenderedDir, subDir))
	if err != nil {
		return nil, err
	}

	list := make([]any, 0, len(des))
	m := map[string]bool{}

	for _, de := range des {
		if de.IsDir() {
			continue
		}

		lname := strings.ToLower(de.Name())
		resourcePath := ""

		if di.isHelmValuesYaml(de.Name()) {
			continue
		} else if di.isHelmChartYaml(de.Name()) {
			hr, err := di.newHelmRelease(subDir)
			if err != nil {
				return nil, err
			}
			if !utils.IsFile(filepath.Join(di.RenderedDir, subDir, hr.GetOutputPath())) {
				resourcePath = hr.GetOutputPath()
			}
		} else if strings.HasSuffix(lname, ".yml") || strings.HasSuffix(lname, ".yaml") {
			resourcePath = de.Name()
		}

		if resourcePath != "" {
			resourcePath = filepath.ToSlash(resourcePath)
			resourcePath = path.Clean(resourcePath)
			if _, ok := m[resourcePath]; !ok {
				m[resourcePath] = true
				list = append(list, resourcePath)
			}
		}
	}

	generated := uo.New()
	_ = generated.SetNestedField(list, "resources")

	return generated, nil
}

func (di *DeploymentItem) writeKustomizationYaml(ky *uo.UnstructuredObject) error {
	kustomizeYamlPath := yaml.FixPathExt(filepath.Join(di.RenderedDir, "kustomization.yml"))
	return yaml.WriteYamlFile(kustomizeYamlPath, ky)
}

func (di *DeploymentItem) prepareKustomizationYaml() (*uo.UnstructuredObject, error) {
	ky, err := di.readKustomizationYaml("")
	if err != nil {
		return nil, err
	}
	if ky == nil {
		ky, err = di.generateKustomizationYaml("")
		if err != nil {
			return nil, err
		}
		err = di.writeKustomizationYaml(ky)
		if err != nil {
			return nil, err
		}
	}

	overrideNamespace := di.Project.getOverrideNamespace()
	if overrideNamespace != nil {
		_, ok, err := ky.GetNestedString("namespace")
		if err != nil {
			return nil, err
		}
		if !ok {
			ky.SetNestedField(*overrideNamespace, "namespace")
		}
	}

	di.Barrier = ky.GetK8sAnnotationBoolNoError("kluctl.io/barrier", false)
	di.WaitReadiness = ky.GetK8sAnnotationBoolNoError("kluctl.io/wait-readiness", false)

	return ky, nil
}

func (di *DeploymentItem) buildKustomize() error {
	if di.dir == nil {
		return nil
	}
	if di.Config.OnlyRender {
		return nil
	}

	ky, err := di.prepareKustomizationYaml()
	if err != nil {
		return err
	}

	// Save modified kustomization.yml
	err = di.writeKustomizationYaml(ky)
	if err != nil {
		return err
	}

	fs, err := securefs.MakeFsOnDiskSecureBuild(di.RenderedSourceRootDir)
	if err != nil {
		return err
	}

	fs = sops.NewDecryptingFs(fs, di.ctx.SopsDecrypter)
	rm, err := kustomize.Build(fs, di.RenderedDir)
	if err != nil {
		return err
	}

	di.Objects = nil
	for _, r := range rm.Resources() {
		y, err := r.Map()
		if err != nil {
			return err
		}
		o := uo.FromMap(y)
		di.Objects = append(di.Objects, o)
	}

	return nil
}

func (di *DeploymentItem) postprocessObjects(images *Images) error {
	if di.dir == nil {
		return nil
	}

	var errs *multierror.Error
	for _, o := range di.Objects {
		commonLabels := di.getCommonLabels()
		commonAnnotations := di.getCommonAnnotations()

		_ = k8s.UnwrapListItems(o, true, func(o *uo.UnstructuredObject) error {
			// Set common labels/annotations
			for n, v := range commonLabels {
				o.SetK8sLabel(n, v)
			}
			for n, v := range commonAnnotations {
				o.SetK8sAnnotation(n, v)
			}

			// Resolve image placeholders
			err := images.ResolvePlaceholders(di.ctx.Ctx, di.ctx.K, o, di.RelRenderedDir, di.Tags.ListKeys(), di.VarsCtx.Vars)
			if err != nil {
				errs = multierror.Append(errs, err)
			}
			return nil
		})
	}

	return errs.ErrorOrNil()
}

func (di *DeploymentItem) collectResultObjects() error {
	for _, o := range di.Objects {
		di.Config.RenderedObjects = append(di.Config.RenderedObjects, o.GetK8sRef())
	}
	return nil
}

func (di *DeploymentItem) writeRenderedYaml() error {
	if di.dir == nil {
		return nil
	}

	var objects []interface{}
	for _, o := range di.Objects {
		objects = append(objects, o.Object)
	}

	// Need to write it back to disk in case it is needed externally
	err := yaml.WriteYamlAllFile(di.renderedYamlPath, objects)
	if err != nil {
		return err
	}
	return nil
}
