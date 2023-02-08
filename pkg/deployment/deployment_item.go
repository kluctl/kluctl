package deployment

import (
	"fmt"
	"github.com/fluxcd/pkg/kustomize"
	"github.com/hashicorp/go-multierror"
	"github.com/kluctl/kluctl/v2/pkg/helm"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/sops"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/vars"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io/fs"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const SealmeExt = ".sealme"

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
	Tags    *utils.OrderedMap

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
	i := 0
	for _, t := range di.Tags.ListKeys() {
		l[fmt.Sprintf("kluctl.io/tag-%d", i)] = t
		i += 1
	}
	return l
}

func (di *DeploymentItem) getCommonAnnotations() map[string]string {
	a := map[string]string{
		"kluctl.io/deployment-item-dir": filepath.ToSlash(di.RelToSourceItemDir),
	}
	if di.Config.SkipDeleteIfTags {
		a["kluctl.io/skip-delete-if-tags"] = "true"
	}
	return a
}

func (di *DeploymentItem) render(forSeal bool) error {
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

	if !forSeal {
		// .sealme files are rendered while sealing and not while deploying
		excludePatterns = append(excludePatterns, "**.sealme")
	}

	searchDirs := di.Project.getRenderSearchDirs()
	// also add deployment item dir to search dirs
	searchDirs = append([]string{*di.dir}, searchDirs...)

	return di.VarsCtx.RenderDirectory(
		filepath.Join(di.Project.source.dir, di.RelToSourceItemDir),
		di.RenderedDir,
		excludePatterns,
		searchDirs,
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
	helmChartsDir := filepath.Join(di.Project.source.dir, ".helm-charts")

	hr, err := helm.NewRelease(di.Project.source.dir, filepath.Join(di.RelToSourceItemDir, subDir), configPath, helmChartsDir, di.ctx.HelmCredentials)
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

		return hr.Render(di.ctx.Ctx, di.ctx.K, di.ctx.K8sVersion, di.ctx.SopsDecrypter)
	})
	if err != nil {
		return err
	}
	return nil
}

func (di *DeploymentItem) ListSealedSecrets(subdir string) ([]string, error) {
	var ret []string

	if di.dir == nil {
		return nil, nil
	}

	renderedDir := filepath.Join(di.RenderedDir, subdir)

	// ensure we're not leaving the project
	err := utils.CheckSubInDir(di.Project.source.dir, subdir)
	if err != nil {
		return nil, err
	}

	y, err := di.readKustomizationYaml(subdir)
	if err != nil {
		return nil, err
	}
	if y == nil {
		y, err = di.generateKustomizationYaml(subdir)
		if err != nil {
			return nil, err
		}
	}

	resources, _, err := y.GetNestedStringList("resources")
	if err != nil {
		return nil, err
	}

	for _, resource := range resources {
		p := filepath.Clean(filepath.Join(renderedDir, resource))

		isDir := utils.IsDirectory(p)
		isSealedSecret := !utils.Exists(p) && utils.Exists(p+SealmeExt)
		if !isDir && !isSealedSecret {
			continue
		}

		relPath, err := filepath.Rel(di.RenderedDir, p)
		if err != nil {
			return nil, err
		}

		relPath = filepath.Clean(relPath)

		// ensure we're not leaving the project
		err = utils.CheckSubInDir(di.Project.source.dir, relPath)
		if err != nil {
			return nil, err
		}

		if isDir {
			ret2, err := di.ListSealedSecrets(relPath)
			if err != nil {
				return nil, err
			}
			ret = append(ret, ret2...)
		} else {
			ret = append(ret, relPath)
		}
	}
	return ret, nil
}

func (di *DeploymentItem) BuildSealedSecretPath(relPath string) (string, error) {
	sealedSecretsDir := di.Project.getRenderedOutputPattern()
	baseSourcePath := di.Project.ctx.SealedSecretsDir

	relDir := filepath.Dir(relPath)
	fname := filepath.Base(relPath)

	// ensure we're not leaving the .sealed-secrets dir
	sourcePath := filepath.Join(baseSourcePath, di.RelRenderedDir, relDir, sealedSecretsDir, fname)
	err := utils.CheckInDir(baseSourcePath, sourcePath)
	if err != nil {
		return "", err
	}
	sourcePath = filepath.Clean(sourcePath)
	return sourcePath, nil
}

func (di *DeploymentItem) resolveSealedSecrets() error {
	if di.dir == nil {
		return nil
	}

	sealedSecrets, err := di.ListSealedSecrets("")
	if err != nil {
		return err
	}

	for _, relPath := range sealedSecrets {
		relDir := filepath.Dir(relPath)
		if err != nil {
			return err
		}
		fname := filepath.Base(relPath)

		baseError := fmt.Sprintf("failed to resolve SealedSecret %s", relPath)

		sourcePath, err := di.BuildSealedSecretPath(relPath)
		if err != nil {
			return fmt.Errorf("%s: %w", baseError, err)
		}

		targetPath := filepath.Join(di.RenderedDir, relDir, fname)
		if !utils.IsFile(sourcePath) {
			return fmt.Errorf("%s. %s not found. You might need to seal it first", baseError, sourcePath)
		}
		b, err := os.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to read source secret file %s: %w", sourcePath, err)
		}
		err = os.WriteFile(targetPath, b, 0o600)
		if err != nil {
			return fmt.Errorf("failed to write target secret file %s: %w", targetPath, err)
		}
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

	var list []any
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
		} else if strings.HasSuffix(lname, ".yml"+SealmeExt) || strings.HasSuffix(lname, ".yaml"+SealmeExt) {
			resourcePath = de.Name()[:len(de.Name())-len(SealmeExt)]
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

	di.Barrier = utils.ParseBoolOrFalse(ky.GetK8sAnnotation("kluctl.io/barrier"))
	di.WaitReadiness = utils.ParseBoolOrFalse(ky.GetK8sAnnotation("kluctl.io/wait-readiness"))

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

	resources, _, _ := ky.GetNestedStringList("resources")
	for _, r := range resources {
		p := filepath.Join(di.RenderedDir, r)
		if utils.IsFile(p) {
			err = sops.MaybeDecryptFile(di.ctx.SopsDecrypter, p)
			if err != nil {
				return err
			}
		}
	}

	rm, err := kustomize.SecureBuild(di.RenderedSourceRootDir, di.RenderedDir, true)
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

var crdGV = schema.GroupKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition"}

// postprocessCRDs will update api resources from freshly deployed CRDs
// value even if the CRD is not deployed yet.
func (di *DeploymentItem) postprocessCRDs() error {
	if di.dir == nil {
		return nil
	}
	if di.ctx.K == nil {
		return nil
	}

	for _, o := range di.Objects {
		gvk := o.GetK8sGVK()
		if gvk.GroupKind() != crdGV {
			continue
		}

		err := di.ctx.K.Resources.UpdateResourcesFromCRD(o)
		if err != nil {
			return err
		}
	}
	return nil
}

func (di *DeploymentItem) postprocessObjects(images *Images) error {
	if di.dir == nil {
		return nil
	}

	var objects []interface{}

	var errs *multierror.Error
	for _, o := range di.Objects {
		commonLabels := di.getCommonLabels()
		commonAnnotations := di.getCommonAnnotations()

		_ = k8s.UnwrapListItems(o, true, func(o *uo.UnstructuredObject) error {
			if di.ctx.K != nil {
				di.ctx.K.Resources.FixNamespace(o, "default")
			}

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

		objects = append(objects, o.Object)
	}

	if errs.ErrorOrNil() != nil {
		return errs.ErrorOrNil()
	}

	// Need to write it back to disk in case it is needed externally
	err := yaml.WriteYamlAllFile(di.renderedYamlPath, objects)
	if err != nil {
		return err
	}

	return nil
}
