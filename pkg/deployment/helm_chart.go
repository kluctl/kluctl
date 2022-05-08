package deployment

import (
	"context"
	"fmt"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
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
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type helmChart struct {
	configFile string
	Config     *types.HelmChartConfig
}

func NewHelmChart(configFile string) (*helmChart, error) {
	var config types.HelmChartConfig
	err := yaml.ReadYamlFile(configFile, &config)
	if err != nil {
		return nil, err
	}

	hc := &helmChart{
		configFile: configFile,
		Config:     &config,
	}
	return hc, nil
}

func (c *helmChart) GetChartName() (string, error) {
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

func (c *helmChart) GetChartDir() (string, error) {
	chartName, err := c.GetChartName()
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(c.configFile)
	targetDir := filepath.Join(dir, "charts")
	return securejoin.SecureJoin(targetDir, chartName)
}

func (c *helmChart) GetOutputPath() (string, error) {
	dir := filepath.Dir(c.configFile)
	return securejoin.SecureJoin(dir, c.Config.Output)
}

func (c *helmChart) buildHelmConfig() (*action.Configuration, error) {
	rc, err := registry.NewClient()
	if err != nil {
		return nil, err
	}
	return &action.Configuration{
		RegistryClient: rc,
	}, nil
}

func (c *helmChart) Pull(ctx context.Context) error {
	chartName, err := c.GetChartName()
	if err != nil {
		return err
	}

	chartDir, err := c.GetChartDir()
	if err != nil {
		return err
	}

	targetDir := filepath.Join(filepath.Dir(c.configFile), "charts")
	_ = os.RemoveAll(chartDir)

	cfg, err := c.buildHelmConfig()
	if err != nil {
		return err
	}
	a := action.NewPullWithOpts(action.WithConfig(cfg))
	a.Settings = cli.New()
	a.Untar = true
	a.DestDir = targetDir
	a.Version = *c.Config.ChartVersion

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

func (c *helmChart) CheckUpdate() (string, bool, error) {
	if c.Config.Repo != nil && registry.IsOCI(*c.Config.Repo) {
		return "", false, nil
	}
	chartName, err := c.GetChartName()
	if err != nil {
		return "", false, err
	}
	var latestVersion string

	settings := cli.New()
	e := repo.Entry{
		URL:  *c.Config.Repo,
		Name: chartName,
	}
	r, err := repo.NewChartRepository(&e, getter.All(settings))
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

	indexEntry, ok := index.Entries[*c.Config.ChartName]
	if !ok || len(indexEntry) == 0 {
		return "", false, fmt.Errorf("helm chart %s not found in repo index", *c.Config.ChartName)
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

func (c *helmChart) Render(ctx context.Context, k *k8s.K8sCluster) error {
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

func (c *helmChart) doRender(ctx context.Context, k *k8s.K8sCluster) error {
	chartDir, err := c.GetChartDir()
	if err != nil {
		return err
	}
	outputPath, err := c.GetOutputPath()
	if err != nil {
		return err
	}
	valuesPath := yaml.FixPathExt(filepath.Join(filepath.Dir(c.configFile), "helm-values.yml"))

	var gvs []string
	if k != nil {
		gvs, err = k.GetAllGroupVersions()
		if err != nil {
			return err
		}
	}
	cfg, err := c.buildHelmConfig()
	if err != nil {
		return err
	}

	settings := cli.New()
	valueOpts := values.Options{
		ValueFiles: []string{valuesPath},
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

	if c.Config.SkipCRDs != nil && *c.Config.SkipCRDs {
		client.SkipCRDs = true
	} else {
		client.IncludeCRDs = true
	}
	for _, gv := range gvs {
		client.APIVersions = append(client.APIVersions, gv)
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

	var parsed []*uo.UnstructuredObject

	doParse := func(s string) error {
		m, err := yaml.ReadYamlAllString(s)
		if err != nil {
			return err
		}
		for _, x := range m {
			x2, ok := x.(map[string]interface{})
			if !ok {
				return fmt.Errorf("yaml object is not a map")
			}
			parsed = append(parsed, uo.FromMap(x2))
		}
		return nil
	}

	err = doParse(rel.Manifest)
	if err != nil {
		return err
	}

	if !client.DisableHooks {
		for _, m := range rel.Hooks {
			if isTestHook(m) {
				continue
			}
			err = doParse(m.Manifest)
			if err != nil {
				return err
			}
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

	err = ioutil.WriteFile(outputPath, rendered, 0o600)
	if err != nil {
		return err
	}
	return nil
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

func (c *helmChart) Save() error {
	return yaml.WriteYamlFile(c.configFile, c.Config)
}
