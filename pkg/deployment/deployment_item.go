package deployment

import (
	"fmt"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io/fs"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"strings"
)

const SealmeExt = ".sealme"

type DeploymentItem struct {
	ctx SharedContext

	Project   *DeploymentProject
	Inclusion *utils.Inclusion
	Config    *types.DeploymentItemConfig
	dir       *string
	index     int

	// These values come from the metadata of the kustomization.yml
	Barrier       bool
	WaitReadiness bool

	Objects []*uo.UnstructuredObject
	Tags    *utils.OrderedMap

	RelToRootItemDir    string
	RelToProjectItemDir string
	relRenderedDir      string
	renderedDir         string
	renderedYamlPath    string
}

func NewDeploymentItem(ctx SharedContext, project *DeploymentProject, collection *DeploymentCollection, config *types.DeploymentItemConfig, dir *string, index int) (*DeploymentItem, error) {
	di := &DeploymentItem{
		ctx:       ctx,
		Project:   project,
		Inclusion: collection.Inclusion,
		Config:    config,
		dir:       dir,
		index:     index,
	}

	var err error

	// collect tags
	di.Tags = di.Project.getTags()
	di.Tags.SetMultiple(di.Config.Tags, true)

	if di.dir != nil {
		di.RelToRootItemDir, err = filepath.Rel(di.Project.rootDir, *di.dir)
		if err != nil {
			return nil, err
		}

		di.RelToProjectItemDir, err = filepath.Rel(di.Project.relDir, di.RelToRootItemDir)
		if err != nil {
			return nil, err
		}

		di.relRenderedDir = di.RelToRootItemDir
		if di.index != 0 {
			di.relRenderedDir = fmt.Sprintf("%s-%d", di.relRenderedDir, di.index)
		}

		di.renderedDir = filepath.Join(collection.ctx.RenderDir, di.relRenderedDir)
		di.renderedYamlPath = filepath.Join(di.renderedDir, ".rendered.yml")
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
	// TODO change it to kluctl.io/deployment_dir
	a := map[string]string{
		"kluctl.io/kustomize_dir": filepath.ToSlash(di.RelToRootItemDir),
	}
	if di.Config.SkipDeleteIfTags {
		a["kluctl.io/skip-delete-if-tags"] = "true"
	}
	return a
}

func (di *DeploymentItem) render(forSeal bool, wp *utils.WorkerPoolWithErrors) error {
	if di.dir == nil {
		return nil
	}

	err := os.MkdirAll(di.renderedDir, 0o700)
	if err != nil {
		return err
	}

	varsCtx := di.Project.VarsCtx.Copy()

	err = di.Project.loadVarsList(varsCtx, di.Config.Vars)
	if err != nil {
		return err
	}

	var excludePatterns []string
	excludePatterns = append(excludePatterns, di.Project.Config.TemplateExcludes...)
	err = filepath.WalkDir(*di.dir, func(p string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			relDir, err := filepath.Rel(*di.dir, p)
			if err != nil {
				return err
			}
			if yaml.Exists(filepath.Join(p, "helm-chart.yml")) {
				// never try to render helm charts
				ep := filepath.Join(di.RelToProjectItemDir, relDir, "charts/**")
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

	wp.Submit(func() error {
		return varsCtx.RenderDirectory(di.Project.rootDir, di.Project.getRenderSearchDirs(), di.Project.relDir, excludePatterns, di.RelToProjectItemDir, di.renderedDir)
	})

	return nil
}

func (di *DeploymentItem) renderHelmCharts(wp *utils.WorkerPoolWithErrors) error {
	if di.dir == nil {
		return nil
	}

	err := filepath.Walk(di.renderedDir, func(p string, info fs.FileInfo, err error) error {
		if !strings.HasSuffix(p, "helm-chart.yml") && !strings.HasSuffix(p, "helm-chart.yaml") {
			return nil
		}

		wp.Submit(func() error {
			subDir, err := filepath.Rel(di.renderedDir, filepath.Dir(p))
			if err != nil {
				return err
			}

			chart, err := NewHelmChart(p)
			if err != nil {
				return err
			}

			ky, err := di.readKustomizationYaml(subDir)
			if err == nil && ky != nil {
				resources, _, _ := ky.GetNestedStringList("resources")
				found := false
				for _, r := range resources {
					if r == chart.GetOutputPath() {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("%s/kustomization.yaml does not include the rendered helm chart: %s", di.relRenderedDir, chart.GetOutputPath())
				}
			}

			return chart.Render(di.ctx.Ctx, di.ctx.K)
		})
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (di *DeploymentItem) resolveSealedSecrets(subdir string) error {
	if di.dir == nil {
		return nil
	}

	sealedSecretsDir := di.Project.getRenderedOutputPattern()
	baseSourcePath := di.Project.ctx.SealedSecretsDir

	renderedDir := filepath.Join(di.renderedDir, subdir)

	// ensure we're not leaving the project
	_, err := securejoin.SecureJoin(di.Project.rootDir, subdir)
	if err != nil {
		return err
	}

	y, err := uo.FromFile(yaml.FixPathExt(filepath.Join(renderedDir, "kustomization.yml")))
	if err != nil {
		return err
	}
	l, _, err := y.GetNestedStringList("resources")
	if err != nil {
		return err
	}
	for _, resource := range l {
		p := filepath.Join(renderedDir, resource)
		if utils.IsDirectory(p) {
			err = di.resolveSealedSecrets(filepath.Join(subdir, resource))
			if err != nil {
				return err
			}
			continue
		}
		if utils.Exists(p) || !utils.Exists(p+SealmeExt) {
			continue
		}
		relDir, err := filepath.Rel(renderedDir, filepath.Dir(p))
		if err != nil {
			return err
		}
		fname := filepath.Base(p)

		baseError := fmt.Sprintf("failed to resolve SealedSecret %s", filepath.Clean(filepath.Join(di.Project.absDir, resource)))

		// ensure we're not leaving the .sealed-secrets dir
		sourcePath, err := securejoin.SecureJoin(baseSourcePath, filepath.Join(di.relRenderedDir, subdir, relDir, sealedSecretsDir, fname))
		if err != nil {
			return err
		}
		sourcePath = filepath.Clean(sourcePath)
		targetPath := filepath.Join(renderedDir, relDir, fname)
		if !utils.IsFile(sourcePath) {
			return fmt.Errorf("%s. %s not found. You might need to seal it first", baseError, sourcePath)
		}
		b, err := ioutil.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to read source secret file %s: %w", sourcePath, err)
		}
		err = ioutil.WriteFile(targetPath, b, 0o600)
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
		dir := filepath.ToSlash(di.RelToRootItemDir)
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
	values := di.buildInclusionEntries()
	return di.Inclusion.CheckIncluded(values, false)
}

func (di *DeploymentItem) checkInclusionForDelete() bool {
	if di.Inclusion == nil {
		return true
	}
	values := di.buildInclusionEntries()
	return di.Inclusion.CheckIncluded(values, di.Config.SkipDeleteIfTags)
}

func (di *DeploymentItem) readKustomizationYaml(subDir string) (*uo.UnstructuredObject, error) {
	if di.dir == nil {
		return nil, nil
	}

	kustomizeYamlPath := yaml.FixPathExt(filepath.Join(di.renderedDir, subDir, "kustomization.yml"))
	if !utils.IsFile(kustomizeYamlPath) {
		return nil, nil
	}

	ky, err := uo.FromFile(kustomizeYamlPath)
	if err != nil {
		return nil, err
	}

	return ky, err
}

func (di *DeploymentItem) writeKustomizationYaml(ky *uo.UnstructuredObject) error {
	kustomizeYamlPath := yaml.FixPathExt(filepath.Join(di.renderedDir, "kustomization.yml"))
	return yaml.WriteYamlFile(kustomizeYamlPath, ky)
}

func (di *DeploymentItem) prepareKustomizationYaml() error {
	ky, err := di.readKustomizationYaml("")
	if err != nil {
		return err
	}
	if ky == nil {
		return nil
	}

	overrideNamespace := di.Project.getOverrideNamespace()
	if overrideNamespace != nil {
		_, ok, err := ky.GetNestedString("namespace")
		if err != nil {
			return err
		}
		if !ok {
			ky.SetNestedField(*overrideNamespace, "namespace")
		}
	}

	di.Barrier = utils.ParseBoolOrFalse(ky.GetK8sAnnotation("kluctl.io/barrier"))
	di.WaitReadiness = utils.ParseBoolOrFalse(ky.GetK8sAnnotation("kluctl.io/wait-readiness"))

	// Save modified kustomize.yml
	return di.writeKustomizationYaml(ky)
}

func (di *DeploymentItem) buildKustomize() error {
	if di.dir == nil {
		return nil
	}

	err := di.prepareKustomizationYaml()
	if err != nil {
		return err
	}

	utils.WaitForOpenapiInitDone()

	ko := krusty.MakeDefaultOptions()
	k := krusty.MakeKustomizer(ko)

	fsys := filesys.MakeFsOnDisk()
	rm, err := k.Run(fsys, di.renderedDir)
	if err != nil {
		return err
	}

	var yamls []interface{}
	for _, r := range rm.Resources() {
		y, err := r.Map()
		if err != nil {
			return err
		}
		yamls = append(yamls, y)
	}

	err = yaml.WriteYamlAllFile(di.renderedYamlPath, yamls)
	if err != nil {
		return err
	}

	return nil
}

func (di *DeploymentItem) loadObjects() error {
	if di.dir == nil {
		return nil
	}

	objects, err := yaml.ReadYamlAllFile(di.renderedYamlPath)
	if err != nil {
		return err
	}

	di.Objects = []*uo.UnstructuredObject{}
	for _, o := range objects {
		m, ok := o.(map[string]interface{})
		if !ok {
			return fmt.Errorf("object is not a map")
		}
		di.Objects = append(di.Objects, uo.FromMap(m))
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

	var errList []error
	for _, o := range di.Objects {
		commonLabels := di.getCommonLabels()
		commonAnnotations := di.getCommonAnnotations()

		_ = k8s.UnwrapListItems(o, true, func(o *uo.UnstructuredObject) error {
			if di.ctx.K != nil {
				di.ctx.K.Resources.FixNamespace(o, "default")
			}

			// Set common labels/annotations
			o.SetK8sLabels(uo.CopyMergeStrMap(o.GetK8sLabels(), commonLabels))
			o.SetK8sAnnotations(uo.CopyMergeStrMap(o.GetK8sAnnotations(), commonAnnotations))

			// Resolve image placeholders
			err := images.ResolvePlaceholders(di.ctx.K, o, di.relRenderedDir, di.Tags.ListKeys())
			if err != nil {
				errList = append(errList, err)
			}
			return nil
		})

		objects = append(objects, o.Object)
	}

	if len(errList) != 0 {
		return utils.NewErrorListOrNil(errList)
	}

	// Need to write it back to disk in case it is needed externally
	err := yaml.WriteYamlAllFile(di.renderedYamlPath, objects)
	if err != nil {
		return err
	}

	return nil
}
