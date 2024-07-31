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
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/release"
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

	if config.IsLocalChart() {
		err := config.ErrWhenLocalPathInvalid()
		if err != nil {
			return nil, err
		}
		localPath, err = config.GetAbsoluteLocalPath(projectRoot, relDirInProject)
		if err != nil {
			return nil, err
		}
	}
	credentialsIdValue := ""
	if config.CredentialsId != nil {
		status.Deprecation(ctx, "helm-release-credentials-id", "'credentialsId' in helm-chart.yaml is deprecated and support for it will be removed in a future version of Kluctl.")
		credentialsIdValue = *config.CredentialsId
	}

	chart, err := NewChart(config.Repo, localPath, config.ChartName, config.Git, helmAuthProvider, credentialsIdValue, ociAuthProvider, gitRp, ociRp)
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
	if hr.Chart.IsRegistryChart() {
		if !hr.Config.SkipPrePull {
			pc, err := hr.Chart.GetPrePulledChart(hr.baseChartsDir, hr.Config.ChartVersion)
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
						"Desired version is %s while pre-pulled version is %s", hr.Chart.GetChartName(), hr.Config.ChartVersion, prePulledVersion)
				} else {
					//goland:noinspection ALL
					return nil, fmt.Errorf("Helm Chart %s has not been pre-pulled, which is not allowed when skipPrePull is not enabled. "+
						"Run 'kluctl helm-pull' to pre-pull all Helm Charts", hr.Chart.GetChartName())
				}
			}

			return pc, nil
		} else {
			s := status.Startf(ctx, "Pulling Helm Chart %s with version %s", hr.Chart.GetChartName(), hr.Config.ChartVersion)
			defer s.Failed()

			pc, err := hr.Chart.PullCached(ctx, hr.Config.ChartVersion)
			if err != nil {
				return nil, err
			}
			s.Success()

			return pc, nil
		}
	}
	if hr.Chart.IsRepositoryChart() {
		var pc *PulledChart
		var err error
		ref, _, err := hr.Config.GetGitRef()
		if !hr.Config.SkipPrePull {
			pc, err = hr.Chart.GetPrePulledChart(hr.baseChartsDir, ref)
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
						"Desired ref is %s while pre-pulled ref is %s", hr.Chart.GetChartName(), ref, prePulledVersion)
				} else {
					//goland:noinspection ALL
					return nil, fmt.Errorf("Helm Chart %s has not been pre-pulled, which is not allowed when skipPrePull is not enabled. "+
						"Run 'kluctl helm-pull' to pre-pull all Helm Charts", hr.Chart.GetChartName())
				}
			}
			return pc, nil
		} else {
			s := status.Startf(ctx, "Pulling Helm Chart %s with branch, tag or commit %s", hr.Chart.GetChartName(), ref)
			defer s.Failed()

			pc, err := hr.Chart.PullCached(ctx, ref)
			if err != nil {
				return nil, err
			}
			s.Success()

			return pc, nil
		}
	}
	return nil, fmt.Errorf("Unkown source of Helm Chart. Please set either path, repo or git.")
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

	cfg, err := buildHelmConfig(k, nil)
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
	if k != nil {
		client.DryRunOption = "server"
	}
	client.Namespace = namespace
	client.ReleaseName = hr.Config.ReleaseName
	client.Replace = true
	client.ClientOnly = true
	client.KubeVersion = kubeVersion
	client.APIVersions, err = hr.getApiVersions(k)
	if err != nil {
		return err
	}

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
		status.Warningf(ctx, "Chart %s is deprecated", hr.Config.ChartName)
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

func (hr *Release) getApiVersions(k *k8s.K8sCluster) (chartutil.VersionSet, error) {
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
