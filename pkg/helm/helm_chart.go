package helm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"github.com/Masterminds/semver/v3"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/registries"
	"github.com/kluctl/kluctl/v2/pkg/sops"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/pkg/errors"
	"github.com/rogpeppe/go-internal/lockedfile"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/chartutil"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/cli/values"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/release"
	"helm.sh/helm/v3/pkg/repo"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type HelmCredentialsProvider interface {
	FindCredentials(repoUrl string, credentialsId *string) *repo.Entry
}

type HelmChart struct {
	ConfigFile string
	Config     *types.HelmChartConfig

	credentials HelmCredentialsProvider

	chartName          string
	chartDir           string
	deprecatedChartDir string

	versions []string
}

func NewHelmChart(baseChartsDir string, configFile string) (*HelmChart, error) {
	var config types.HelmChartConfig
	err := yaml.ReadYamlFile(configFile, &config)
	if err != nil {
		return nil, err
	}

	_, err = semver.NewVersion(*config.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid chart version '%s': %w", *config.ChartVersion, err)
	}

	hc := &HelmChart{
		ConfigFile: configFile,
		Config:     &config,
	}

	if hc.Config.Repo != nil && registry.IsOCI(*hc.Config.Repo) {
		s := strings.Split(*hc.Config.Repo, "/")
		chartName := s[len(s)-1]
		if m, _ := regexp.MatchString(`[a-zA-Z_-]+`, chartName); !m {
			return nil, fmt.Errorf("invalid oci chart url: %s", *hc.Config.Repo)
		}
		hc.chartName = chartName
	} else if hc.Config.ChartName == nil {
		return nil, fmt.Errorf("chartName is missing in helm-chart.yml")
	} else {
		hc.chartName = *hc.Config.ChartName
	}

	dir := filepath.Dir(configFile)
	chartDir := filepath.Join(dir, "charts")
	chartDir, err = securejoin.SecureJoin(chartDir, hc.chartName)
	if err != nil {
		return nil, err
	}
	hc.deprecatedChartDir = chartDir

	hc.chartDir, err = hc.buildChartDir(baseChartsDir)
	if err != nil {
		return nil, err
	}

	return hc, nil
}

func (c *HelmChart) buildChartDir(baseChartsDir string) (string, error) {
	u, err := url.Parse(*c.Config.Repo)
	if err != nil {
		return "", err
	}

	scheme := ""
	port := ""
	switch {
	case registry.IsOCI(*c.Config.Repo):
		scheme = "oci"
	case u.Scheme == "http":
		scheme = "http"
		if u.Port() != "80" {
			port = u.Port()
		}
	case u.Scheme == "https":
		scheme = "https"
		if u.Port() != "443" {
			port = u.Port()
		}
	default:
		return "", fmt.Errorf("unsupported scheme in %s", u.String())
	}
	if port != "" {
		scheme += "_" + port
	}

	dir := filepath.Join(
		baseChartsDir,
		fmt.Sprintf("%s_%s", scheme, strings.ToLower(u.Hostname())),
		filepath.FromSlash(strings.ToLower(u.Path)),
		c.chartName,
	)
	err = utils.CheckInDir(baseChartsDir, dir)
	if err != nil {
		return "", err
	}

	return dir, nil
}

func (c *HelmChart) GetChartName() string {
	return c.chartName
}

func (c *HelmChart) GetChartDir(withVersion bool) string {
	if withVersion {
		return filepath.Join(c.chartDir, *c.Config.ChartVersion)
	}
	return c.chartDir
}

func (c *HelmChart) GetDeprecatedChartDir() string {
	return c.deprecatedChartDir
}

func (c *HelmChart) GetOutputPath() string {
	output := "helm-rendered.yaml"
	if c.Config.Output != nil {
		output = *c.Config.Output
	}
	return output
}

func (c *HelmChart) GetFullOutputPath() (string, error) {
	dir := filepath.Dir(c.ConfigFile)
	return securejoin.SecureJoin(dir, c.GetOutputPath())
}

func (c *HelmChart) buildHelmConfig(k *k8s.K8sCluster) (*action.Configuration, error) {
	rc, err := registry.NewClient()
	if err != nil {
		return nil, err
	}

	return &action.Configuration{
		RESTClientGetter: k,
		RegistryClient:   rc,
	}, nil
}

func (c *HelmChart) checkNeedsPull(chartDir string, isTmp bool) (bool, bool, string, error) {
	if !utils.IsDirectory(chartDir) {
		return true, false, "", nil
	}

	chartYamlPath := yaml.FixPathExt(filepath.Join(chartDir, "Chart.yaml"))
	st, err := os.Stat(chartYamlPath)
	if err != nil {
		if os.IsNotExist(err) {
			return true, false, "", nil
		}
		return false, false, "", err
	}
	if isTmp && time.Now().Sub(st.ModTime()) >= time.Hour*24 {
		// MacOS will delete tmp files after 3 days, so lets be safe and re-pull every day
		return true, false, "", nil
	}

	chartYaml, err := uo.FromFile(chartYamlPath)
	if err != nil {
		return false, false, "", err
	}

	version, _, _ := chartYaml.GetNestedString("version")
	if version != *c.Config.ChartVersion {
		return true, true, version, nil
	}
	return false, false, "", nil
}

func (c *HelmChart) Pull(ctx context.Context) error {
	if utils.Exists(c.GetDeprecatedChartDir()) {
		err := os.RemoveAll(c.GetDeprecatedChartDir())
		if err != nil {
			return err
		}
	}

	return c.doPull(ctx, c.GetChartDir(true))
}

func (c *HelmChart) pullTmpChart(ctx context.Context) (string, error) {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s\n", *c.Config.Repo)
	_, _ = fmt.Fprintf(hash, "%s\n", c.chartName)
	_, _ = fmt.Fprintf(hash, "%s\n", *c.Config.ChartVersion)
	h := hex.EncodeToString(hash.Sum(nil))
	tmpDir := filepath.Join(utils.GetTmpBaseDir(ctx), "helm-charts")
	_ = os.MkdirAll(tmpDir, 0o755)
	tmpDir = filepath.Join(tmpDir, fmt.Sprintf("%s-%s", c.chartName, h))

	lockFile := tmpDir + ".lock"
	lock, err := lockedfile.Create(lockFile)
	if err != nil {
		return "", err
	}
	defer lock.Close()

	needsPull, _, _, err := c.checkNeedsPull(tmpDir, true)
	if err != nil {
		return "", err
	}
	if !needsPull {
		return tmpDir, nil
	}
	err = c.doPull(ctx, tmpDir)
	if err != nil {
		return "", err
	}
	return tmpDir, nil
}

func (c *HelmChart) doPull(ctx context.Context, chartDir string) error {
	baseDir := filepath.Dir(chartDir)

	_ = os.RemoveAll(chartDir)
	_ = os.MkdirAll(baseDir, 0o755)

	// need to use the same filesystem/volume that we later os.Rename the final pull chart to, as otherwise
	// the rename operation will lead to errors
	tmpDir, err := os.MkdirTemp(baseDir, c.chartName+"-pull-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tmpChartDir := filepath.Join(tmpDir, c.chartName)

	cfg, err := c.buildHelmConfig(nil)
	if err != nil {
		return err
	}
	a := action.NewPullWithOpts(action.WithConfig(cfg))
	a.Settings = cli.New()
	a.Untar = true
	a.DestDir = tmpDir
	a.Version = *c.Config.ChartVersion

	if c.Config.CredentialsId != nil {
		if registry.IsOCI(*c.Config.Repo) {
			return fmt.Errorf("OCI charts can currently only be authenticated via registry login and not via cli arguments")
		}
		if c.credentials == nil {
			return fmt.Errorf("no credentials provider")
		}
	}

	if c.credentials != nil {
		creds := c.credentials.FindCredentials(*c.Config.Repo, c.Config.CredentialsId)
		if creds != nil {
			a.Username = creds.Username
			a.Password = creds.Password
			a.CertFile = creds.CertFile
			a.CaFile = creds.CAFile
			a.KeyFile = creds.KeyFile
			a.InsecureSkipTLSverify = creds.InsecureSkipTLSverify
			a.PassCredentialsAll = creds.PassCredentialsAll
		}
	}

	var out string
	if registry.IsOCI(*c.Config.Repo) {
		out, err = a.Run(*c.Config.Repo)
	} else {
		a.RepoURL = *c.Config.Repo
		out, err = a.Run(c.chartName)
	}
	if err != nil {
		return err
	}

	// a bug in the Pull command causes this directory to be created by accident
	_ = os.RemoveAll(tmpChartDir + fmt.Sprintf("-%s.tar.gz", a.Version))
	_ = os.RemoveAll(tmpChartDir + fmt.Sprintf("-%s.tgz", a.Version))
	if out != "" {
		status.PlainText(ctx, out)
	}

	err = os.Rename(tmpChartDir, chartDir)
	if err != nil {
		return err
	}

	return nil
}

func (c *HelmChart) QueryLatestVersion(ctx context.Context) (string, error) {
	if c.Config.Repo != nil && registry.IsOCI(*c.Config.Repo) {
		return c.queryLatestVersionOci(ctx)
	}
	return c.queryLatestVersionHelmRepo(ctx)
}

func (c *HelmChart) queryLatestVersionOci(ctx context.Context) (string, error) {
	rh := registries.NewRegistryHelper(ctx)

	imageName := strings.TrimPrefix(*c.Config.Repo, "oci://")
	tags, err := rh.ListImageTags(imageName)
	if err != nil {
		return "", err
	}

	latestVersion, err := c.findLatestVersion(tags)
	if err != nil {
		return "", err
	}

	return latestVersion, nil
}

func (c *HelmChart) queryLatestVersionHelmRepo(ctx context.Context) (string, error) {
	settings := cli.New()

	var e *repo.Entry
	if c.Config.CredentialsId != nil {
		if c.credentials == nil {
			return "", fmt.Errorf("no credentials provider")
		}
		e = c.credentials.FindCredentials(*c.Config.Repo, c.Config.CredentialsId)
		if e == nil {
			return "", fmt.Errorf("no credentials provided for Chart %s", c.chartName)
		}
	} else {
		e = &repo.Entry{
			URL: *c.Config.Repo,
		}
	}

	r, err := repo.NewChartRepository(e, getter.All(settings))
	if err != nil {
		return "", err
	}

	r.CachePath, err = os.MkdirTemp(utils.GetTmpBaseDir(ctx), "helm-check-update-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(r.CachePath)

	indexFile, err := r.DownloadIndexFile()
	if err != nil {
		return "", err
	}

	index, err := repo.LoadIndexFile(indexFile)
	if err != nil {
		return "", err
	}

	indexEntry, ok := index.Entries[c.chartName]
	if !ok || len(indexEntry) == 0 {
		return "", fmt.Errorf("helm chart %s not found in repo index", c.chartName)
	}

	versions := make([]string, 0, indexEntry.Len())
	for _, x := range indexEntry {
		versions = append(versions, x.Version)
	}

	latestVersion, err := c.findLatestVersion(versions)
	if err != nil {
		return "", err
	}

	return latestVersion, nil
}

func (c *HelmChart) findLatestVersion(inputVersions []string) (string, error) {
	var err error
	var updateConstraints *semver.Constraints
	if c.Config.UpdateConstraints != nil {
		updateConstraints, err = semver.NewConstraint(*c.Config.UpdateConstraints)
		if err != nil {
			return "", fmt.Errorf("invalid update constraints '%s': %w", *c.Config.UpdateConstraints, err)
		}
	}

	var versions semver.Collection
	for _, x := range inputVersions {
		v, err := semver.NewVersion(x)
		if err != nil {
			continue
		}

		if updateConstraints == nil {
			if v.Prerelease() != "" {
				// we don't allow pre-releases by default. To allow pre-releases, use 1.0.0-0 as constraint
				continue
			}
		} else if !updateConstraints.Check(v) {
			continue
		}
		versions = append(versions, v)
	}
	if len(versions) == 0 {
		if c.Config.UpdateConstraints == nil {
			return "", fmt.Errorf("no version found")
		} else {
			return "", fmt.Errorf("no version found that satisfies constraints '%s'", *c.Config.UpdateConstraints)
		}
	}

	sort.Stable(versions)
	latestVersion := versions[len(versions)-1].Original()
	return latestVersion, nil
}

func (c *HelmChart) Render(ctx context.Context, k *k8s.K8sCluster, k8sVersion string, sopsDecrypter sops.SopsDecrypter) error {
	deprecatedChartDir := c.GetDeprecatedChartDir()
	if utils.IsDirectory(deprecatedChartDir) {
		status.Deprecation(ctx, "helm-charts-dir", "Your project has pre-pulled charts located beside the helm-chart.yaml, which is deprecated. "+
			"Please run 'kluctl helm-pull' on your project and ensure that the deprecated charts are removed! Future versions of kluctl will ignore these locations.")
	}

	err := c.doRender(ctx, k, k8sVersion, sopsDecrypter)
	if err != nil {
		return fmt.Errorf("rendering helm chart %s for release %s has failed: %w", c.chartName, c.Config.ReleaseName, err)
	}
	return nil
}

func (c *HelmChart) doRender(ctx context.Context, k *k8s.K8sCluster, k8sVersion string, sopsDecrypter sops.SopsDecrypter) error {
	chartDir := c.deprecatedChartDir

	needsPull, versionChanged, prePulledVersion, err := c.checkNeedsPull(chartDir, false)
	if err != nil {
		return err
	}
	if needsPull {
		if versionChanged {
			return fmt.Errorf("pre-pulled Helm Chart %s need to be pulled (call 'kluctl helm-pull'). "+
				"Desired version is %s while pre-pulled version is %s", c.chartName, *c.Config.ChartVersion, prePulledVersion)
		} else {
			status.Warning(ctx, "Warning, need to pull Helm Chart %s with version %s. "+
				"Please consider pre-pulling it with 'kluctl helm-pull'", c.chartName, *c.Config.ChartVersion)
		}

		s := status.Start(ctx, "Pulling Helm Chart %s with version %s", c.chartName, *c.Config.ChartVersion)
		defer s.Failed()

		chartDir, err = c.pullTmpChart(ctx)
		if err != nil {
			return err
		}
		s.Success()
	}

	outputPath, err := c.GetFullOutputPath()
	if err != nil {
		return err
	}
	valuesPath := yaml.FixPathExt(filepath.Join(filepath.Dir(c.ConfigFile), "helm-values.yml"))

	var gvs []schema.GroupVersion
	if k != nil {
		gvs, err = k.Resources.GetAllGroupVersions()
		if err != nil {
			return err
		}
	}
	cfg, err := c.buildHelmConfig(k)
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
	if c.Config.Namespace != nil {
		namespace = *c.Config.Namespace
	}

	client := action.NewInstall(cfg)
	client.DryRun = true
	client.Namespace = namespace
	client.ReleaseName = c.Config.ReleaseName
	client.Replace = true
	client.ClientOnly = true
	client.KubeVersion = kubeVersion

	if c.Config.SkipCRDs {
		client.SkipCRDs = true
	} else {
		client.IncludeCRDs = true
	}
	for _, gv := range gvs {
		client.APIVersions = append(client.APIVersions, gv.String())
	}

	p := getter.All(settings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return err
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(chartDir)
	if err != nil {
		return err
	}

	if err := checkIfInstallable(chartRequested); err != nil {
		return err
	}

	if chartRequested.Metadata.Deprecated {
		status.Warning(ctx, "Chart %s is deprecated", *c.Config.ChartName)
	}

	rel, err := client.Run(chartRequested, vals)
	if err != nil {
		return err
	}

	parsed, err := c.parseRenderedManifests(rel.Manifest)
	if err != nil {
		return err
	}

	if !client.DisableHooks {
		for _, m := range rel.Hooks {
			if isTestHook(m) {
				continue
			}
			parsedHooks, err := c.parseRenderedManifests(m.Manifest)
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
				k.Resources.FixNamespace(o, namespace)
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

func (c *HelmChart) parseRenderedManifests(s string) ([]*uo.UnstructuredObject, error) {
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

func (c *HelmChart) SetCredentials(p HelmCredentialsProvider) {
	c.credentials = p
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

func (c *HelmChart) Save() error {
	return yaml.WriteYamlFile(c.ConfigFile, c.Config)
}
