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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"path"
	"path/filepath"
	"sigs.k8s.io/kustomize/api/filesys"
	"sigs.k8s.io/kustomize/api/krusty"
	"strings"
)

const sealmeExt = ".sealme"

type deploymentItem struct {
	project    *DeploymentProject
	collection *DeploymentCollection
	config     *types.DeploymentItemConfig
	dir        *string
	index      int

	objects []*unstructured.Unstructured

	relProjectDir       string
	relToRootItemDir    string
	relToProjectItemDir string
	relRenderedDir      string
	renderedDir         string
	renderedYamlPath    string
}

func NewDeploymentItem(project *DeploymentProject, collection *DeploymentCollection, config *types.DeploymentItemConfig, dir *string, index int) (*deploymentItem, error) {
	di := &deploymentItem{
		project:    project,
		collection: collection,
		config:     config,
		dir:        dir,
		index:      index,
	}

	var err error

	rootProject := di.project.getRootProject()

	di.relProjectDir, err = filepath.Rel(rootProject.dir, di.project.dir)
	if err != nil {
		return nil, err
	}

	if di.dir != nil {
		di.relToRootItemDir, err = filepath.Rel(rootProject.dir, *di.dir)
		if err != nil {
			return nil, err
		}

		di.relToProjectItemDir, err = filepath.Rel(di.relProjectDir, di.relToRootItemDir)
		if err != nil {
			return nil, err
		}

		di.relRenderedDir = di.relToRootItemDir
		if di.index != 0 {
			di.relRenderedDir = fmt.Sprintf("%s-%d", di.relRenderedDir, di.index)
		}

		di.renderedDir = path.Join(di.collection.RenderDir, di.relRenderedDir)
		di.renderedYamlPath = path.Join(di.renderedDir, ".rendered.yml")
	}
	return di, nil
}

func (di *deploymentItem) getCommonLabels() map[string]string {
	l := di.project.getCommonLabels()
	i := 0
	for _, t := range di.getTags().ListKeys() {
		l[fmt.Sprintf("kluctl.io/tag-%d", i)] = t
		i += 1
	}
	return l
}

func (di *deploymentItem) getCommonAnnotations() map[string]string {
	// TODO change it to kluctl.io/deployment_dir
	a := map[string]string{
		"kluctl.io/kustomize_dir": strings.ReplaceAll(di.relToRootItemDir, string(os.PathSeparator), "/"),
	}
	if di.config.SkipDeleteIfTags != nil && *di.config.SkipDeleteIfTags {
		a["kluctl.io/skip-delete-if-tags"] = "true"
	}
	return a
}

func (di *deploymentItem) render(k *k8s.K8sCluster, wp *utils.WorkerPoolWithErrors) error {
	if di.dir == nil {
		return nil
	}

	rootDir := di.project.getRootProject().dir

	err := os.MkdirAll(di.renderedDir, 0o777)
	if err != nil {
		return err
	}

	varsCtx := di.project.varsCtx.Copy()
	err = varsCtx.loadVarsList(k, di.project.getRenderSearchDirs(), di.config.Vars)
	if err != nil {
		return err
	}

	var excludePatterns []string
	excludePatterns = append(excludePatterns, di.project.config.TemplateExcludes...)
	if utils.IsFile(path.Join(*di.dir, "helm-chart.yml")) {
		// never try to render helm charts
		excludePatterns = append(excludePatterns, path.Join(di.relToProjectItemDir, "charts/**"))
	}
	if !di.collection.forSeal {
		// .sealme files are rendered while sealing and not while deploying
		excludePatterns = append(excludePatterns, "**.sealme")
	}

	wp.Submit(func() error {
		return varsCtx.renderDirectory(rootDir, di.project.getRenderSearchDirs(), di.relProjectDir, excludePatterns, di.relToProjectItemDir, di.renderedDir)
	})

	return nil
}

func (di *deploymentItem) renderHelmCharts(k *k8s.K8sCluster, wp *utils.WorkerPoolWithErrors) error {
	if di.dir == nil {
		return nil
	}

	err := filepath.Walk(di.renderedDir, func(p string, info fs.FileInfo, err error) error {
		if !strings.HasSuffix(p, "helm-chart.yml") {
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

func (di *deploymentItem) resolveSealedSecrets() error {
	if di.dir == nil {
		return nil
	}

	// TODO check for bootstrap

	sealedSecretsDir := di.project.getSealedSecretsDir()
	baseSourcePath := di.project.sealedSecretsDir

	var y map[string]interface{}
	err := yaml.ReadYamlFile(path.Join(di.renderedDir, "kustomization.yml"), &y)
	if err != nil {
		return err
	}
	l, _, err := unstructured.NestedStringSlice(y, "resources")
	if err != nil {
		return err
	}
	for _, resource := range l {
		p := path.Join(di.renderedDir, resource)
		if utils.Exists(p) || !utils.Exists(p+sealmeExt) {
			continue
		}
		relDir, err := filepath.Rel(di.renderedDir, path.Dir(p))
		if err != nil {
			return err
		}
		fname := path.Base(p)

		baseError := fmt.Sprintf("failed to resolve SealedSecret %s", path.Clean(path.Join(di.project.dir, resource)))
		if sealedSecretsDir == nil {
			return fmt.Errorf("%s. Sealed secrets dir could not be determined", baseError)
		}
		sourcePath := path.Clean(path.Join(baseSourcePath, di.relRenderedDir, relDir, *sealedSecretsDir, fname))
		targetPath := path.Join(di.renderedDir, relDir, fname)
		if !utils.IsFile(sourcePath) {
			return fmt.Errorf("%s. %s not found. You might need to seal it first", baseError, sourcePath)
		}
		b, err := ioutil.ReadFile(sourcePath)
		if err != nil {
			return fmt.Errorf("failed to read source secret file %s: %w", sourcePath, err)
		}
		err = ioutil.WriteFile(targetPath, b, 0o666)
		if err != nil {
			return fmt.Errorf("failed to write target secret file %s: %w", targetPath, err)
		}
	}
	return nil
}

func (di *deploymentItem) getTags() *utils.OrderedMap {
	tags := di.project.getTags()
	for _, t := range di.config.Tags {
		tags.Set(t, true)
	}
	return tags
}

func (di *deploymentItem) buildInclusionEntries() []utils.InclusionEntry {
	var values []utils.InclusionEntry
	for _, t := range di.getTags().ListKeys() {
		values = append(values, utils.InclusionEntry{Type: "tag", Value: t})
	}
	if di.dir != nil {
		dir := strings.ReplaceAll(di.relToProjectItemDir, string(os.PathSeparator), "/")
		values = append(values, utils.InclusionEntry{Type: "kustomize_dir", Value: dir})
	}
	return values
}

func (di *deploymentItem) checkInclusionForDeploy() bool {
	if di.collection.inclusion == nil {
		return true
	}
	if di.config.OnlyRender != nil && *di.config.OnlyRender {
		return true
	}
	if di.config.AlwaysDeploy != nil && *di.config.AlwaysDeploy {
		return true
	}
	values := di.buildInclusionEntries()
	return di.collection.inclusion.CheckIncluded(values, false)
}

func (di *deploymentItem) checkInclusionForDelete() bool {
	if di.collection.inclusion == nil {
		return true
	}
	skipDeleteIfTags := di.config.SkipDeleteIfTags != nil && *di.config.SkipDeleteIfTags
	values := di.buildInclusionEntries()
	return di.collection.inclusion.CheckIncluded(values, skipDeleteIfTags)
}

func (di *deploymentItem) prepareKusomizationYaml() error {
	if di.dir == nil {
		return nil
	}

	kustomizeYamlPath := path.Join(di.renderedDir, "kustomization.yml")
	if !utils.IsFile(kustomizeYamlPath) {
		return nil
	}

	var kustomizeYaml map[string]interface{}
	err := yaml.ReadYamlFile(kustomizeYamlPath, &kustomizeYaml)
	if err != nil {
		return err
	}

	overrideNamespace := di.project.getOverrideNamespace()
	if overrideNamespace != nil {
		if _, ok := kustomizeYaml["namespace"]; !ok {
			kustomizeYaml["namespace"] = *overrideNamespace
		}
	}

	// Save modified kustomize.yml
	err = yaml.WriteYamlFile(kustomizeYamlPath, kustomizeYaml)
	if err != nil {
		return err
	}
	return nil
}

func (di *deploymentItem) buildKustomize() error {
	if di.dir == nil {
		return nil
	}

	err := di.prepareKusomizationYaml()
	if err != nil {
		return err
	}

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

func (di *deploymentItem) postprocessAndLoadObjects(k *k8s.K8sCluster) error {
	if di.dir == nil {
		return nil
	}

	objects, err := yaml.ReadYamlAllFile(di.renderedYamlPath)
	if err != nil {
		return err
	}

	di.objects = []*unstructured.Unstructured{}
	for _, o := range objects {
		m, ok := o.(map[string]interface{})
		if !ok {
			return fmt.Errorf("object is not a map")
		}
		di.objects = append(di.objects, &unstructured.Unstructured{
			Object: m,
		})
	}

	for _, o := range di.objects {
		if k != nil {
			k.RemoveNamespaceIfNeeded(o)
		}

		// Set common labels/annotations
		o.SetLabels(uo.CopyMergeStrMap(o.GetLabels(), di.getCommonLabels()))
		commonAnnotations := di.getCommonAnnotations()
		o.SetAnnotations(uo.CopyMergeStrMap(o.GetAnnotations(), commonAnnotations))

		// Resolve image placeholders
		err = di.collection.images.ResolvePlaceholders(k, o, di.relRenderedDir, di.getTags().ListKeys())
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
