package deployment

import (
	"context"
	"fmt"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/utils/versions"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"github.com/pkg/errors"
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
)

type HelmChart struct {
	ConfigFile  string
	Config      *types.HelmChartConfig
	credentials *repo.Entry
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
	return hc, nil
}

func (c *HelmChart) GetChartName() (string, error) {
	if c.Config.Repo != nil && registry.IsOCI(*c.Config.Repo) {
		s := strings.Split(*c.Config.Repo, "/")
		chartName := s[len(s)-1]
		if m, _ := regexp.MatchString(`[a-zA-Z_-]+`, chartName); !m {
			return "", fmt.Errorf("invalid oci chart url: %s", *c.Config.Repo)
		}
		return chartName, nil
	}
	if c.Config.ChartName == nil {
		return "", fmt.Errorf("chartName is missing in helm-chart.yml")
	}
	return *c.Config.ChartName, nil
}

func (c *HelmChart) GetChartDir() (string, error) {
	chartName, err := c.GetChartName()
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(c.ConfigFile)
	targetDir := filepath.Join(dir, "charts")
	return securejoin.SecureJoin(targetDir, chartName)
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

func (c *HelmChart) Pull(ctx context.Context) error {
	chartName, err := c.GetChartName()
	if err != nil {
		return err
	}

	chartDir, err := c.GetChartDir()
	if err != nil {
		return err
	}

	targetDir := filepath.Join(filepath.Dir(c.ConfigFile), "charts")
	_ = os.RemoveAll(chartDir)

	cfg, err := c.buildHelmConfig(nil)
	if err != nil {
		return err
	}
	a := action.NewPullWithOpts(action.WithConfig(cfg))
	a.Settings = cli.New()
	a.Untar = true
	a.DestDir = targetDir
	a.Version = *c.Config.ChartVersion

	if c.credentials != nil {
		a.Username = c.credentials.Username
		a.Password = c.credentials.Password
		a.CertFile = c.credentials.CertFile
		a.CaFile = c.credentials.CAFile
		a.KeyFile = c.credentials.KeyFile
		a.InsecureSkipTLSverify = c.credentials.InsecureSkipTLSverify
		a.PassCredentialsAll = c.credentials.PassCredentialsAll
	}

	var out string
	if registry.IsOCI(*c.Config.Repo) {
		out, err = a.Run(*c.Config.Repo)
	} else {
		a.RepoURL = *c.Config.Repo
		out, err = a.Run(chartName)
	}
	// a bug in the Pull command causes this directory to be created by accident
	_ = os.RemoveAll(chartDir + fmt.Sprintf("-%s.tar.gz", a.Version))
	_ = os.RemoveAll(chartDir + fmt.Sprintf("-%s.tgz", a.Version))
	if out != "" {
		status.PlainText(ctx, out)
	}
	if err != nil {
		return err
	}
	return nil
}

func (c *HelmChart) CheckUpdate() (string, bool, error) {
	if c.Config.Repo != nil && registry.IsOCI(*c.Config.Repo) {
		return "", false, nil
	}
	chartName, err := c.GetChartName()
	if err != nil {
		return "", false, err
	}
	var latestVersion string

	settings := cli.New()
	e := c.credentials
	if e == nil {
		e = &repo.Entry{
			URL: *c.Config.Repo,
		}
	}

	r, err := repo.NewChartRepository(e, getter.All(settings))
	if err != nil {
		return "", false, err
	}

	indexFile, err := r.DownloadIndexFile()
	if err != nil {
		return "", false, err
	}

	index, err := repo.LoadIndexFile(indexFile)
	if err != nil {
		return "", false, err
	}

	indexEntry, ok := index.Entries[chartName]
	if !ok || len(indexEntry) == 0 {
		return "", false, fmt.Errorf("helm chart %s not found in repo index", chartName)
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
	chartName, err := c.GetChartName()
	if err != nil {
		return err
	}
	err = c.doRender(ctx, k)
	if err != nil {
		return fmt.Errorf("rendering helm chart %s for release %s has failed: %w", chartName, c.Config.ReleaseName, err)
	}
	return nil
}

func (c *HelmChart) doRender(ctx context.Context, k *k8s.K8sCluster) error {
	chartDir, err := c.GetChartDir()
	if err != nil {
		return err
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

func (c *HelmChart) SetCredentials(credentials *repo.Entry) {
	c.credentials = credentials
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
