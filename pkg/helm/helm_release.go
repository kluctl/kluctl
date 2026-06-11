package helm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/lib/yaml"
	helmauth "github.com/kluctl/kluctl/v2/pkg/helm/auth"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	ociauth "github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/sops"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/pkg/errors"
	"helm.sh/helm/v4/pkg/action"
	"helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/cli"
	"helm.sh/helm/v4/pkg/cli/values"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/release"
)

const InstallNamespaceAnnotation = "kluctl.io/helm-install-namespace"

type Release struct {
	ConfigFile string
	Config     *types.HelmChartConfig
	Chart      *Chart

	baseChartsDir string
}

func NewRelease(ctx context.Context, projectRoot string, relDirInProject string, configFile string, baseChartsDir string, helmAuthProvider helmauth.HelmAuthProvider, ociAuthProvider ociauth.OciAuthProvider, gitRp *repocache.GitRepoCache, ociRp *repocache.OciRepoCache) (*Release, error) {
	var config types.HelmChartConfig
	var localPath string
	err := yaml.ReadYamlFile(configFile, &config)
	if err != nil {
		return nil, err
	}

	if isLocalChart(config) {
		err := errWhenLocalPathInvalid(config)
		if err != nil {
			return nil, err
		}
		localPath, err = getAbsoluteLocalPath(projectRoot, relDirInProject, config)
		if err != nil {
			return nil, err
		}
	}

	chart, err := NewChart(config.Repo, localPath, config.ChartName, config.Git, helmAuthProvider, ociAuthProvider, gitRp, ociRp)
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

func (hr *Release) GetAbstractVersion() ChartVersion {
	if hr.Chart.IsRegistryChart() {
		return ChartVersion{
			Version: hr.Config.ChartVersion,
		}
	} else if hr.Chart.IsGitRepositoryChart() {
		return ChartVersion{
			GitRef: hr.Config.Git.Ref,
		}
	}
	panic("neither chart version nor tag, commit or branch are defined")
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
		localChartVersion, err := hr.Chart.GetLocalChartVersion()
		version := ChartVersion{
			Version: &localChartVersion,
		}
		if err != nil {
			return nil, err
		}
		return NewPulledChart(hr.Chart, version, hr.Chart.GetLocalPath(), false), nil
	}

	var version ChartVersion
	if hr.Chart.IsGitRepositoryChart() {
		version.GitRef = hr.Config.Git.Ref
	} else if hr.Chart.IsRegistryChart() {
		version.Version = hr.Config.ChartVersion
	} else {
		return nil, fmt.Errorf("unkown source of Helm Chart. Please set either path, repo or git")
	}

	if !hr.Config.SkipPrePull {
		pc, err := hr.Chart.GetPrePulledChart(hr.baseChartsDir, version)
		if err != nil {
			return nil, err
		}

		needsPull, versionChanged, prePulledVersion, err := pc.CheckNeedsPull()
		if err != nil {
			return nil, err
		}

		if needsPull {
			if versionChanged {
				return nil, fmt.Errorf("pre-pulled Helm Chart %s need to be pulled (call 'kluctl helm-pull'). "+
					"Desired version is %s while pre-pulled version is %s", hr.Chart.GetChartName(), version.String(), prePulledVersion.String())
			} else {
				//goland:noinspection ALL
				return nil, fmt.Errorf("Helm Chart %s has not been pre-pulled, which is not allowed when skipPrePull is not enabled. "+
					"Run 'kluctl helm-pull' to pre-pull all Helm Charts", hr.Chart.GetChartName())
			}
		}

		return pc, nil
	} else {
		s := status.Startf(ctx, "Pulling Helm Chart %s with version %s", hr.Chart.GetChartName(), hr.GetAbstractVersion().String())
		defer s.Failed()

		pc, err := hr.Chart.PullCached(ctx, version)
		if err != nil {
			return nil, err
		}
		s.Success()

		return pc, nil
	}
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

	apiVersions, err := hr.getApiVersions(k)
	if err != nil {
		return err
	}

	cfg, err := buildHelmConfig(k, nil, k8sVersion, apiVersions)
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

	namespace := "default"
	if hr.Config.Namespace != nil {
		namespace = *hr.Config.Namespace
	}

	client := action.NewInstall(cfg)
	client.KubeVersion = &cfg.Capabilities.KubeVersion
	client.DryRunStrategy = action.DryRunClient
	if k != nil {
		client.DryRunStrategy = action.DryRunServer
	}
	client.Namespace = namespace
	client.ReleaseName = hr.Config.ReleaseName
	client.Replace = true
	client.TakeOwnership = true

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
	chartAccessor, err := chart.NewAccessor(chartRequested)
	if err != nil {
		return err
	}

	if err := checkIfInstallable(chartAccessor); err != nil {
		return err
	}

	if chartAccessor.Deprecated() {
		status.Warningf(ctx, "Chart %s is deprecated", hr.Config.ChartName)
	}

	rel, err := client.Run(chartRequested, vals)
	if err != nil {
		return err
	}

	relAccessor, err := release.NewAccessor(rel)
	if err != nil {
		return err
	}

	parsed, err := hr.parseRenderedManifests(relAccessor.Manifest())
	if err != nil {
		return err
	}

	if !client.DisableHooks {
		for _, h := range relAccessor.Hooks() {
			if ish, err := isTestHook(h); err == nil && ish {
				continue
			} else if err != nil {
				return err
			}
			ha, err := release.NewHookAccessor(h)
			if err != nil {
				return err
			}
			parsedHooks, err := hr.parseRenderedManifests(ha.Manifest())
			if err != nil {
				return err
			}
			parsed = append(parsed, parsedHooks...)
		}
	}

	parsedI := make([]any, 0, len(parsed))
	for _, o := range parsed {
		if hr.Config.Namespace != nil {
			// mark it for later namespace fixing
			o.SetK8sAnnotation(InstallNamespaceAnnotation, *hr.Config.Namespace)
		}
		parsedI = append(parsedI, o)
	}

	rendered, err := yaml.WriteYamlAllBytes(parsedI)
	if err != nil {
		return err
	}

	err = os.WriteFile(outputPath, rendered, 0o600)
	if err != nil {
		return err
	}
	return nil
}

func (hr *Release) getApiVersions(k *k8s.K8sCluster) (common.VersionSet, error) {
	if k == nil {
		return nil, nil
	}

	m := map[string]bool{}

	ars, err := k.GetAllAPIResources()
	if err != nil {
		return nil, err
	}
	for _, ar := range ars {
		gvStr := ar.Version
		if ar.Group != "" {
			gvStr = ar.Group + "/" + ar.Version
		}
		m[gvStr] = true
		gvkStr := fmt.Sprintf("%s/%s", gvStr, ar.Kind)
		m[gvkStr] = true
	}

	ret := make([]string, 0, len(m))
	for id, _ := range m {
		ret = append(ret, id)
	}

	return ret, nil
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

func checkIfInstallable(ch chart.Accessor) error {
	m := ch.MetadataAsMap()
	switch m["Type"] {
	case "", "application":
		return nil
	}
	return errors.Errorf("%s charts are not installable", m["Type"])
}

func isTestHook(h release.Hook) (bool, error) {
	a, err := release.NewHookAccessor(h)
	if err != nil {
		return false, err
	}
	o, err := uo.FromString(a.Manifest())
	if err != nil {
		return false, err
	}
	eventsStr := o.GetK8sAnnotation("helm.sh/hook")
	if eventsStr == nil {
		return false, nil
	}
	events := strings.Split(*eventsStr, ",")
	for _, event := range events {
		if strings.TrimSpace(event) == "test" {
			return true, nil
		}
	}
	return false, nil
}

func isLocalChart(config types.HelmChartConfig) bool {
	return config.Path != ""
}
func errWhenLocalPathInvalid(config types.HelmChartConfig) error {
	if filepath.IsAbs(config.Path) {
		return fmt.Errorf("absolute path is not allowed in helm-chart.yaml")
	}
	return nil
}

func getAbsoluteLocalPath(projectRoot string, relDirInProject string, config types.HelmChartConfig) (string, error) {
	localPath := ""
	localPath = filepath.Join(projectRoot, relDirInProject, config.Path)
	localPath, err := filepath.Abs(localPath)
	if err != nil {
		return "", err
	}
	err = utils.CheckInDir(projectRoot, localPath)
	if err != nil {
		return "", err
	}
	return localPath, nil
}
