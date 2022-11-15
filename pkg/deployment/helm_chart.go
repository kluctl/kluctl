package deployment

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/registries"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/utils/versions"
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

	chartName string
	chartDir  string
}

func NewHelmChart(configFile string) (*HelmChart, error) {
	var config types.HelmChartConfig
	err := yaml.ReadYamlFile(configFile, &config)
	if err != nil {
		return nil, err
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
	hc.chartDir = chartDir

	return hc, nil
}

func (c *HelmChart) GetChartName() string {
	return c.chartName
}

func (c *HelmChart) GetChartDir() string {
	return c.chartDir
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
	return c.doPull(ctx, c.chartDir)
}

func (c *HelmChart) pullTmpChart(ctx context.Context) (string, error) {
	hash := sha256.New()
	_, _ = fmt.Fprintf(hash, "%s\n", *c.Config.Repo)
	_, _ = fmt.Fprintf(hash, "%s\n", c.chartName)
	_, _ = fmt.Fprintf(hash, "%s\n", *c.Config.ChartVersion)
	h := hex.EncodeToString(hash.Sum(nil))
	tmpDir := filepath.Join(utils.GetTmpBaseDir(), "helm-charts")
	_ = os.MkdirAll(tmpDir, 0o700)
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
	_ = os.RemoveAll(chartDir)
	_ = os.MkdirAll(filepath.Dir(chartDir), 0o700)

	tmpDir, err := os.MkdirTemp(utils.GetTmpBaseDir(), "helm-pull-")
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
		if c.credentials == nil {
			return fmt.Errorf("no credentials provider")
		}
		creds := c.credentials.FindCredentials(*c.Config.Repo, c.Config.CredentialsId)
		if creds == nil {
			return fmt.Errorf("no credentials provided for Chart %s", c.chartName)
		}
		a.Username = creds.Username
		a.Password = creds.Password
		a.CertFile = creds.CertFile
		a.CaFile = creds.CAFile
		a.KeyFile = creds.KeyFile
		a.InsecureSkipTLSverify = creds.InsecureSkipTLSverify
		a.PassCredentialsAll = creds.PassCredentialsAll
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

func (c *HelmChart) CheckUpdate(ctx context.Context) (string, bool, error) {
	if c.Config.Repo != nil && registry.IsOCI(*c.Config.Repo) {
		return c.checkUpdateOciRepo(ctx)
	}
	return c.checkUpdateHelmRepo()
}

func (c *HelmChart) checkUpdateOciRepo(ctx context.Context) (string, bool, error) {
	rh := registries.NewRegistryHelper(ctx)

	imageName := strings.TrimPrefix(*c.Config.Repo, "oci://")
	tags, err := rh.ListImageTags(imageName)
	if err != nil {
		return "", false, err
	}

	var ls versions.LooseVersionSlice
	for _, x := range tags {
		ls = append(ls, versions.LooseVersion(x))
	}
	sort.Stable(ls)
	latestVersion := string(ls[len(ls)-1])

	updated := latestVersion != *c.Config.ChartVersion
	return latestVersion, updated, nil
}

func (c *HelmChart) checkUpdateHelmRepo() (string, bool, error) {
	var latestVersion string

	settings := cli.New()

	var e *repo.Entry
	if c.Config.CredentialsId != nil {
		if c.credentials == nil {
			return "", false, fmt.Errorf("no credentials provider")
		}
		e = c.credentials.FindCredentials(*c.Config.Repo, c.Config.CredentialsId)
		if e == nil {
			return "", false, fmt.Errorf("no credentials provided for Chart %s", c.chartName)
		}
	} else {
		e = &repo.Entry{
			URL: *c.Config.Repo,
		}
	}

	r, err := repo.NewChartRepository(e, getter.All(settings))
	if err != nil {
		return "", false, err
	}

	r.CachePath, err = os.MkdirTemp(utils.GetTmpBaseDir(), "helm-check-update-")
	if err != nil {
		return "", false, err
	}
	defer os.RemoveAll(r.CachePath)

	indexFile, err := r.DownloadIndexFile()
	if err != nil {
		return "", false, err
	}

	index, err := repo.LoadIndexFile(indexFile)
	if err != nil {
		return "", false, err
	}

	indexEntry, ok := index.Entries[c.chartName]
	if !ok || len(indexEntry) == 0 {
		return "", false, fmt.Errorf("helm chart %s not found in repo index", c.chartName)
	}

	var ls versions.LooseVersionSlice
	for _, x := range indexEntry {
		ls = append(ls, versions.LooseVersion(x.Version))
	}
	sort.Stable(ls)
	latestVersion = string(ls[len(ls)-1])

	updated := latestVersion != *c.Config.ChartVersion
	return latestVersion, updated, nil
}

func (c *HelmChart) Render(ctx context.Context, k *k8s.K8sCluster) error {
	err := c.doRender(ctx, k)
	if err != nil {
		return fmt.Errorf("rendering helm chart %s for release %s has failed: %w", c.chartName, c.Config.ReleaseName, err)
	}
	return nil
}

func (c *HelmChart) doRender(ctx context.Context, k *k8s.K8sCluster) error {
	chartDir := c.chartDir

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
		valueOpts.ValueFiles = append(valueOpts.ValueFiles, valuesPath)
	}

	var kubeVersion *chartutil.KubeVersion
	if k != nil {
		kubeVersion, err = chartutil.ParseKubeVersion(k.ServerVersion.String())
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
