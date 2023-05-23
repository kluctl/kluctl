package helm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Masterminds/semver/v3"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/sops"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/pkg/errors"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
)

type Release struct {
	ConfigFile string
	Config     *types.HelmChartConfig
	Chart      *Chart

	baseChartsDir string
}

func NewRelease(projectRoot string, relDirInProject string, configFile string, baseChartsDir string, credentialsProvider HelmCredentialsProvider) (*Release, error) {
	var config types.HelmChartConfig
	err := yaml.ReadYamlFile(configFile, &config)
	if err != nil {
		return nil, err
	}

	if config.ChartVersion != "" {
		_, err = semver.NewVersion(config.ChartVersion)
		if err != nil {
			return nil, fmt.Errorf("invalid chart version '%s': %w", config.ChartVersion, err)
		}
	}

	localPath := ""
	if config.Path != "" {
		if filepath.IsAbs(config.Path) {
			return nil, fmt.Errorf("absolute path is not allowed in helm-chart.yaml")
		}
		localPath = filepath.Join(projectRoot, relDirInProject, config.Path)
		localPath, err = filepath.Abs(localPath)
		if err != nil {
			return nil, err
		}
		err = utils.CheckInDir(projectRoot, localPath)
		if err != nil {
			return nil, err
		}
	}

	credentialsId := ""
	if config.CredentialsId != nil {
		credentialsId = *config.CredentialsId
	}
	chart, err := NewChart(config.Repo, localPath, config.ChartName, credentialsProvider, credentialsId)
	if err != nil {
		return nil, err
	}

	hr := &Release{
		ConfigFile:    configFile,
		Config:        &config,
		baseChartsDir: baseChartsDir,
		Chart:         chart,
	}

	return hr, nil
}

func (hr *Release) GetOutputPath() string {
	output := "helm-rendered.yaml"
	if hr.Config.Output != nil {
		output = *hr.Config.Output
	}
	return output
}

func (hr *Release) GetFullOutputPath() (string, error) {
	dir := filepath.Dir(hr.ConfigFile)
	return securejoin.SecureJoin(dir, hr.GetOutputPath())
}

func (hr *Release) Render(ctx context.Context, k *k8s.K8sCluster, k8sVersion string, sopsDecrypter *decryptor.Decryptor) error {
	err := hr.doRender(ctx, k, k8sVersion, sopsDecrypter)
	if err != nil {
		return fmt.Errorf("rendering helm chart %s for release %s has failed: %w", hr.Chart.GetChartName(), hr.Config.ReleaseName, err)
	}
	return nil
}

func (hr *Release) getPulledChart(ctx context.Context) (*PulledChart, error) {
	if hr.Chart.IsLocalChart() {
		version, err := hr.Chart.GetLocalChartVersion()
		if err != nil {
			return nil, err
		}
		return NewPulledChart(hr.Chart, version, hr.Chart.GetLocalPath(), false), nil
	}

	pc, err := hr.Chart.GetPulledChart(hr.baseChartsDir, hr.Config.ChartVersion)
	if err != nil {
		return nil, err
	}

	needsPull, versionChanged, prePulledVersion, err := pc.CheckNeedsPull()
	if err != nil {
		return nil, err
	}
	if needsPull {
		if !hr.Config.SkipPrePull {
			//goland:noinspection ALL
			return nil, fmt.Errorf("Helm Chart %s has not been pre-pulled, which is not allowed when skipPrePull is not enabled. "+
				"Run 'kluctl helm-pull' to pre-pull all Helm Charts", hr.Chart.GetChartName())
		}
		if versionChanged {
			return nil, fmt.Errorf("pre-pulled Helm Chart %s need to be pulled (call 'kluctl helm-pull'). "+
				"Desired version is %s while pre-pulled version is %s", hr.Chart.GetChartName(), hr.Config.ChartVersion, prePulledVersion)
		}

		s := status.Start(ctx, "Pulling Helm Chart %s with version %s", hr.Chart.GetChartName(), hr.Config.ChartVersion)
		defer s.Failed()

		pc, err = hr.Chart.PullCached(ctx, hr.Config.ChartVersion)
		if err != nil {
			return nil, err
		}
		s.Success()
	}

	return pc, nil
}

func (hr *Release) doRender(ctx context.Context, k *k8s.K8sCluster, k8sVersion string, sopsDecrypter *decryptor.Decryptor) error {
	pc, err := hr.getPulledChart(ctx)
	if err != nil {
		return err
	}

	outputPath, err := hr.GetFullOutputPath()
	if err != nil {
		return err
	}
	valuesPath := yaml.FixPathExt(filepath.Join(filepath.Dir(hr.ConfigFile), "helm-values.yml"))

	cfg, err := buildHelmConfig(k)
	if err != nil {
		return err
	}

	settings := cli.New()
	valueOpts := values.Options{}

	if utils.Exists(valuesPath) {
		tmpValues, err := sops.MaybeDecryptFileToTmp(ctx, sopsDecrypter, valuesPath)
		if err != nil {
			return err
		}
		defer os.Remove(tmpValues)
		valueOpts.ValueFiles = append(valueOpts.ValueFiles, tmpValues)
	}

	var kubeVersion *chartutil.KubeVersion
	if k != nil {
		kubeVersion, err = chartutil.ParseKubeVersion(k.ServerVersion.String())
		if err != nil {
			return err
		}
	}
	if k8sVersion != "" {
		kubeVersion, err = chartutil.ParseKubeVersion(k8sVersion)
		if err != nil {
			return err
		}
	}

	namespace := "default"
	if hr.Config.Namespace != nil {
		namespace = *hr.Config.Namespace
	}

	client := action.NewInstall(cfg)
	client.DryRun = true
	client.Namespace = namespace
	client.ReleaseName = hr.Config.ReleaseName
	client.Replace = true
	client.ClientOnly = true
	client.KubeVersion = kubeVersion
	client.APIVersions = hr.getApiVersions(k)

	if hr.Config.SkipCRDs {
		client.SkipCRDs = true
	} else {
		client.IncludeCRDs = true
	}

	p := getter.All(settings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return err
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(pc.dir)
	if err != nil {
		return err
	}

	if err := checkIfInstallable(chartRequested); err != nil {
		return err
	}

	if chartRequested.Metadata.Deprecated {
		status.Warning(ctx, "Chart %s is deprecated", hr.Config.ChartName)
	}

	rel, err := client.Run(chartRequested, vals)
	if err != nil {
		return err
	}

	parsed, err := hr.parseRenderedManifests(rel.Manifest)
	if err != nil {
		return err
	}

	if !client.DisableHooks {
		for _, m := range rel.Hooks {
			if isTestHook(m) {
				continue
			}
			parsedHooks, err := hr.parseRenderedManifests(m.Manifest)
			if err != nil {
				return err
			}
			parsed = append(parsed, parsedHooks...)
		}
	}

	var fixed []interface{}
	for _, o := range parsed {
		// "helm install" will deploy resources to the given namespace automatically, but "helm template" does not
		// add the necessary namespace in the rendered resources
		if k != nil {
			err = k8s.UnwrapListItems(o, true, func(o *uo.UnstructuredObject) error {
				k.FixNamespace(o, namespace)
				return nil
			})
			if err != nil {
				return err
			}
		}
		fixed = append(fixed, o)
	}
	rendered, err := yaml.WriteYamlAllBytes(fixed)
	if err != nil {
		return err
	}

	err = os.WriteFile(outputPath, rendered, 0o600)
	if err != nil {
		return err
	}
	return nil
}

func (hr *Release) getApiVersions(k *k8s.K8sCluster) chartutil.VersionSet {
	if k == nil {
		return nil
	}

	m := map[string]bool{}

	gvks := k.Resources.GetFilteredGVKs(nil)
	for _, gvk := range gvks {
		gvStr := gvk.GroupVersion().String()
		m[gvStr] = true
		gvkStr := fmt.Sprintf("%s/%s", gvStr, gvk.Kind)
		m[gvkStr] = true
	}

	ret := make([]string, 0, len(m))
	for id, _ := range m {
		ret = append(ret, id)
	}

	return ret
}

func (hr *Release) parseRenderedManifests(s string) ([]*uo.UnstructuredObject, error) {
	var parsed []*uo.UnstructuredObject

	duplicatesRemoved, err := yaml.RemoveDuplicateFields(strings.NewReader(s))
	if err != nil {
		return nil, err
	}

	m, err := yaml.ReadYamlAllBytes(duplicatesRemoved)
	if err != nil {
		return nil, err
	}
	for _, x := range m {
		x2, ok := x.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("yaml object is not a map")
		}
		parsed = append(parsed, uo.FromMap(x2))
	}
	return parsed, nil
}

func (hr *Release) Save() error {
	return yaml.WriteYamlFile(hr.ConfigFile, hr.Config)
}

func checkIfInstallable(ch *chart.Chart) error {
	switch ch.Metadata.Type {
	case "", "application":
		return nil
	}
	return errors.Errorf("%s charts are not installable", ch.Metadata.Type)
}

func isTestHook(h *release.Hook) bool {
	for _, e := range h.Events {
		if e == release.HookTest {
			return true
		}
	}
	return false
}
