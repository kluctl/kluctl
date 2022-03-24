package deployment

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/codablock/kluctl/pkg/yaml"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/openapi"
	yaml2 "sigs.k8s.io/kustomize/kyaml/yaml"
	"strings"
	"sync"
)

const SealmeExt = ".sealme"

type DeploymentItem struct {
	Project   *DeploymentProject
	Inclusion *utils.Inclusion
	Config    *types.DeploymentItemConfig
	dir       *string
	index     int

	// These values come from the metadata of the kustomization.yml
	Barrier       bool
	WaitReadiness bool

	Objects []*uo.UnstructuredObject

	relProjectDir       string
	relToRootItemDir    string
	RelToProjectItemDir string
	relRenderedDir      string
	renderedDir         string
	renderedYamlPath    string
}

func NewDeploymentItem(project *DeploymentProject, collection *DeploymentCollection, config *types.DeploymentItemConfig, dir *string, index int) (*DeploymentItem, error) {
	di := &DeploymentItem{
		Project:   project,
		Inclusion: collection.Inclusion,
		Config:    config,
		dir:       dir,
		index:     index,
	}

	var err error

	rootProject := di.Project.getRootProject()

	di.relProjectDir, err = filepath.Rel(rootProject.dir, di.Project.dir)
	if err != nil {
		return nil, err
	}

	if di.dir != nil {
		di.relToRootItemDir, err = filepath.Rel(rootProject.dir, *di.dir)
		if err != nil {
			return nil, err
		}

		di.RelToProjectItemDir, err = filepath.Rel(di.relProjectDir, di.relToRootItemDir)
		if err != nil {
			return nil, err
		}

		di.relRenderedDir = di.relToRootItemDir
		if di.index != 0 {
			di.relRenderedDir = fmt.Sprintf("%s-%d", di.relRenderedDir, di.index)
		}

		di.renderedDir = filepath.Join(collection.RenderDir, di.relRenderedDir)
		di.renderedYamlPath = filepath.Join(di.renderedDir, ".rendered.yml")
	}
	return di, nil
}

func (di *DeploymentItem) getCommonLabels() map[string]string {
	l := di.Project.GetCommonLabels()
	i := 0
	for _, t := range di.getTags().ListKeys() {
		l[fmt.Sprintf("kluctl.io/tag-%d", i)] = t
		i += 1
	}
	return l
}

func (di *DeploymentItem) getCommonAnnotations() map[string]string {
	// TODO change it to kluctl.io/deployment_dir
	a := map[string]string{
		"kluctl.io/kustomize_dir": strings.ReplaceAll(di.relToRootItemDir, string(os.PathSeparator), "/"),
	}
	if di.Config.SkipDeleteIfTags != nil && *di.Config.SkipDeleteIfTags {
		a["kluctl.io/skip-delete-if-tags"] = "true"
	}
	return a
}

func (di *DeploymentItem) render(k *k8s.K8sCluster, forSeal bool, wp *utils.WorkerPoolWithErrors) error {
	if di.dir == nil {
		return nil
	}

	rootDir := di.Project.getRootProject().dir

	err := os.MkdirAll(di.renderedDir, 0o700)
	if err != nil {
		return err
	}

	varsCtx := di.Project.VarsCtx.Copy()
	err = varsCtx.LoadVarsList(k, di.Project.getRenderSearchDirs(), di.Config.Vars)
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
				ep = strings.ReplaceAll(ep, string(os.PathSeparator), "/")
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
		return varsCtx.RenderDirectory(rootDir, di.Project.getRenderSearchDirs(), di.relProjectDir, excludePatterns, di.RelToProjectItemDir, di.renderedDir)
	})

	return nil
}

func (di *DeploymentItem) renderHelmCharts(k *k8s.K8sCluster, wp *utils.WorkerPoolWithErrors) error {
	if di.dir == nil {
		return nil
	}

	err := filepath.Walk(di.renderedDir, func(p string, info fs.FileInfo, err error) error {
		if !strings.HasSuffix(p, "helm-chart.yml") && !strings.HasSuffix(p, "helm-chart.yaml") {
			return nil
		}

		wp.Submit(func() error {
			chart, err := NewHelmChart(p)
			if err != nil {
				return err
			}
			return chart.Render(k)
		})
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func (di *DeploymentItem) resolveSealedSecrets() error {
	if di.dir == nil {
		return nil
	}

	// TODO check for bootstrap

	sealedSecretsDir := di.Project.getSealedSecretsDir()
	baseSourcePath := di.Project.SealedSecretsDir

	y, err := uo.FromFile(yaml.FixPathExt(filepath.Join(di.renderedDir, "kustomization.yml")))
	if err != nil {
		return err
	}
	l, _, err := y.GetNestedStringList("resources")
	if err != nil {
		return err
	}
	for _, resource := range l {
		p := filepath.Join(di.renderedDir, resource)
		if utils.Exists(p) || !utils.Exists(p+SealmeExt) {
			continue
		}
		relDir, err := filepath.Rel(di.renderedDir, filepath.Dir(p))
		if err != nil {
			return err
		}
		fname := filepath.Base(p)

		baseError := fmt.Sprintf("failed to resolve SealedSecret %s", filepath.Clean(filepath.Join(di.Project.dir, resource)))
		if sealedSecretsDir == nil {
			return fmt.Errorf("%s. Sealed secrets dir could not be determined", baseError)
		}
		sourcePath := filepath.Clean(filepath.Join(baseSourcePath, di.relRenderedDir, relDir, *sealedSecretsDir, fname))
		targetPath := filepath.Join(di.renderedDir, relDir, fname)
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

func (di *DeploymentItem) getTags() *utils.OrderedMap {
	tags := di.Project.getTags()
	for _, t := range di.Config.Tags {
		tags.Set(t, true)
	}
	return tags
}

func (di *DeploymentItem) buildInclusionEntries() []utils.InclusionEntry {
	var values []utils.InclusionEntry
	for _, t := range di.getTags().ListKeys() {
		values = append(values, utils.InclusionEntry{Type: "tag", Value: t})
	}
	if di.dir != nil {
		dir := strings.ReplaceAll(di.relToRootItemDir, string(os.PathSeparator), "/")
		values = append(values, utils.InclusionEntry{Type: "deploymentItemDir", Value: dir})
	}
	return values
}

func (di *DeploymentItem) CheckInclusionForDeploy() bool {
	if di.Inclusion == nil {
		return true
	}
	if di.Config.OnlyRender != nil && *di.Config.OnlyRender {
		return true
	}
	if di.Config.AlwaysDeploy != nil && *di.Config.AlwaysDeploy {
		return true
	}
	values := di.buildInclusionEntries()
	return di.Inclusion.CheckIncluded(values, false)
}

func (di *DeploymentItem) checkInclusionForDelete() bool {
	if di.Inclusion == nil {
		return true
	}
	skipDeleteIfTags := di.Config.SkipDeleteIfTags != nil && *di.Config.SkipDeleteIfTags
	values := di.buildInclusionEntries()
	return di.Inclusion.CheckIncluded(values, skipDeleteIfTags)
}

func (di *DeploymentItem) prepareKustomizationYaml() error {
	if di.dir == nil {
		return nil
	}

	kustomizeYamlPath := yaml.FixPathExt(filepath.Join(di.renderedDir, "kustomization.yml"))
	if !utils.IsFile(kustomizeYamlPath) {
		return nil
	}

	ky, err := uo.FromFile(kustomizeYamlPath)
	if err != nil {
		return err
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
	err = yaml.WriteYamlFile(kustomizeYamlPath, ky)
	if err != nil {
		return err
	}
	return nil
}

func (di *DeploymentItem) buildKustomize() error {
	if di.dir == nil {
		return nil
	}

	err := di.prepareKustomizationYaml()
	if err != nil {
		return err
	}

	waitForOpenapiInitDone()

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

func (di *DeploymentItem) postprocessAndLoadObjects(k *k8s.K8sCluster, images *Images) error {
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

	for _, o := range di.Objects {
		if k != nil {
			k.RemoveNamespaceIfNeeded(o)
		}

		// Set common labels/annotations
		o.SetK8sLabels(uo.CopyMergeStrMap(o.GetK8sLabels(), di.getCommonLabels()))
		commonAnnotations := di.getCommonAnnotations()
		o.SetK8sAnnotations(uo.CopyMergeStrMap(o.GetK8sAnnotations(), commonAnnotations))

		// Resolve image placeholders
		err = images.ResolvePlaceholders(k, o, di.relRenderedDir, di.getTags().ListKeys())
		if err != nil {
			return err
		}
	}

	// Need to write it back to disk in case it is needed externally
	err = yaml.WriteYamlAllFile(di.renderedYamlPath, objects)
	if err != nil {
		return err
	}

	return nil
}

var openapiInitDoneMutex sync.Mutex
var openapiInitDoneOnce sync.Once

func waitForOpenapiInitDone() {
	openapiInitDoneOnce.Do(func() {
		openapiInitDoneMutex.Lock()
		openapiInitDoneMutex.Unlock()
	})
}

func init() {
	openapiInitDoneMutex.Lock()
	go func() {
		// we do a single call to IsNamespaceScoped to enforce openapi schema initialization
		// this is required here to ensure that it is later not done in parallel which would cause race conditions
		openapi.IsNamespaceScoped(yaml2.TypeMeta{
			APIVersion: "",
			Kind:       "ConfigMap",
		})
		openapiInitDoneMutex.Unlock()
	}()
}
