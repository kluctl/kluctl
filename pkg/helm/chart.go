package helm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/kluctl/kluctl/lib/git/types"
	"github.com/kluctl/kluctl/lib/status"
	"github.com/kluctl/kluctl/lib/yaml"
	helmauth "github.com/kluctl/kluctl/v2/pkg/helm/auth"
	ociauth "github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	cp "github.com/otiai10/copy"
	"github.com/rogpeppe/go-internal/lockedfile"
	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/registry"
	"helm.sh/helm/v3/pkg/repo"
)

type Chart struct {
	repo             string
	localPath        string
	chartName        string
	git              *types.GitInfo
	helmAuthProvider helmauth.HelmAuthProvider
	ociAuthProvider  ociauth.OciAuthProvider
	gitRp            *repocache.GitRepoCache
	ociRp            *repocache.OciRepoCache

	credentialsId string

	versions []string
}

func NewChart(repo string, localPath string, chartName string, git *types.GitInfo, helmAuthProvider helmauth.HelmAuthProvider, credentialsId string, ociAuthProvider ociauth.OciAuthProvider, gitRp *repocache.GitRepoCache, ociRp *repocache.OciRepoCache) (*Chart, error) {
	hc := &Chart{
		repo:             repo,
		git:              git,
		localPath:        localPath,
		helmAuthProvider: helmAuthProvider,
		credentialsId:    credentialsId,
		ociAuthProvider:  ociAuthProvider,
		gitRp:            gitRp,
		ociRp:            ociRp,
	}

	if localPath == "" && repo == "" && git == nil {
		return nil, fmt.Errorf("repo, localPath and git are missing")
	}

	if hc.IsLocalChart() {
		chartYaml, err := uo.FromFile(yaml.FixPathExt(filepath.Join(hc.localPath, "Chart.yaml")))
		if err != nil {
			return nil, err
		}
		x, _, err := chartYaml.GetNestedString("name")
		if err != nil {
			return nil, err
		}
		if x == "" {
			return nil, fmt.Errorf("invalid/empty chart name")
		}
		hc.chartName = x
	} else if registry.IsOCI(repo) {
		if chartName != "" {
			return nil, fmt.Errorf("chartName can't be specified when using OCI repos")
		}
		s := strings.Split(repo, "/")
		chartName := s[len(s)-1]
		if m, _ := regexp.MatchString(`[a-zA-Z_-]+`, chartName); !m {
			return nil, fmt.Errorf("invalid oci chart url: %s", repo)
		}
		hc.chartName = chartName
	} else if hc.IsGitRepositoryChart() {
		if chartName != "" {
			return nil, fmt.Errorf("chartName can't be specified when using git repos")
		}
		s := strings.Split(hc.git.Url.String(), "/")
		chartName := strings.Join(s[len(s)-2:len(s)], "-")
		if m, _ := regexp.MatchString(`[a-zA-Z_-]+`, chartName); !m {
			return nil, fmt.Errorf("invalid git url: %s", hc.git.Url.String())
		}
		hc.chartName = chartName

	} else if chartName == "" {
		return nil, fmt.Errorf("chartName is missing")
	} else {
		hc.chartName = chartName
	}

	return hc, nil
}

func (c *Chart) GetRepo() string {
	return c.repo
}

func (c *Chart) GetLocalPath() string {
	return c.localPath
}

func (c *Chart) IsLocalChart() bool {
	return c.localPath != ""
}

func (c *Chart) IsRegistryChart() bool {
	return c.repo != ""
}

func (c *Chart) IsGitRepositoryChart() bool {
	return c.git != nil
}

func (c *Chart) GetGitRef() (string, string, error) {
	if c.git.Ref.Branch != "" {
		return c.git.Ref.Branch, "branch", nil
	}
	if c.git.Ref.Tag != "" {
		return c.git.Ref.Tag, "tag", nil
	}

	if c.git.Ref.Commit != "" {
		return c.git.Ref.Commit, "commit", nil
	}
	return "", "", fmt.Errorf("neither branch, tag nor commit defined")
}

func (c *Chart) GetLocalChartVersion() (string, error) {
	chartYaml, err := uo.FromFile(yaml.FixPathExt(filepath.Join(c.localPath, "Chart.yaml")))
	if err != nil {
		return "", err
	}
	x, _, err := chartYaml.GetNestedString("version")
	if err != nil {
		return "", err
	}
	if x == "" {
		return "", fmt.Errorf("invalid/empty chart version")
	}
	return x, nil
}

func (c *Chart) BuildRegistryPulledChartDir(baseDir string) (string, error) {
	u, err := url.Parse(c.repo)
	if err != nil {
		return "", err
	}

	scheme := ""
	port := ""
	switch {
	case u.Scheme == "oci":
		scheme = "oci"
		port = u.Port()
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
		baseDir,
		fmt.Sprintf("%s_%s", scheme, strings.ToLower(u.Hostname())),
		filepath.FromSlash(strings.ToLower(u.Path)),
	)
	if u.Scheme != "oci" {
		dir = filepath.Join(dir, c.chartName)
	}
	return dir, nil
}

func (c *Chart) BuildVersionedRegistryPulledChartDir(baseDir string, version string) (string, error) {
	dir, err := c.BuildRegistryPulledChartDir(baseDir)
	if err != nil {
		return "", err
	}
	if version != "" {
		dir = filepath.Join(dir, version)
	}
	return dir, nil
}

func (c *Chart) BuildGitRepositoryPulledChartDir(baseDir string) (string, error) {
	scheme := c.git.Url.Scheme
	port := c.git.Url.NormalizePort()
	hostname := c.git.Url.Hostname()
	path := c.git.Url.Path
	subDir := c.git.SubDir
	if port != "" {
		scheme += "_" + port
	}
	dir := filepath.Join(
		baseDir,
		fmt.Sprintf("%s_%s", scheme, strings.ToLower(hostname)),
		filepath.FromSlash(strings.ToLower(path)),
		filepath.FromSlash(strings.ToLower(subDir)),
	)
	return dir, nil
}

func (c *Chart) BuildVersionedRepositoryPulledChartDir(baseDir string, version string) (string, error) {
	dir, err := c.BuildGitRepositoryPulledChartDir(baseDir)
	if err != nil {
		return "", err
	}
	dir = filepath.Join(dir, version)
	return dir, nil
}

func (c *Chart) BuildPulledChartDir(baseDir string) (string, error) {
	var dir string
	var err error
	if baseDir == "" {
		return "", fmt.Errorf("can't determine pulled chart dir. No base dir for charts found")
	}
	if c.IsRegistryChart() {
		dir, err = c.BuildRegistryPulledChartDir(baseDir)
		if err != nil {
			return "", err
		}
	} else if c.IsGitRepositoryChart() {
		dir, err = c.BuildGitRepositoryPulledChartDir(baseDir)
		if err != nil {
			return "", err
		}
	}
	err = utils.CheckInDir(baseDir, dir)
	if err != nil {
		return "", err
	}
	return dir, nil
}

func (c *Chart) BuildVersionedPulledChartDir(baseDir string, version string) (string, error) {
	var dir string
	var err error
	if baseDir == "" {
		return "", fmt.Errorf("can't determine pulled chart dir. No base dir for charts found")
	}
	if c.IsRegistryChart() {
		dir, err = c.BuildVersionedRegistryPulledChartDir(baseDir, version)
		if err != nil {
			return "", err
		}
	} else if c.IsGitRepositoryChart() {
		dir, err = c.BuildVersionedRepositoryPulledChartDir(baseDir, version)
		if err != nil {
			return "", err
		}
	}
	err = utils.CheckInDir(baseDir, dir)
	if err != nil {
		return "", err
	}
	return dir, nil
}

func (c *Chart) GetChartName() string {
	return c.chartName
}

func (c *Chart) newRegistryClient(ctx context.Context, settings *cli.EnvSettings) (*registry.Client, func(), error) {
	cleanup := func() {}

	u, err := url.Parse(c.repo)
	if err != nil {
		return nil, nil, err
	}

	opts := []registry.ClientOption{
		registry.ClientOptDebug(settings.Debug),
		registry.ClientOptEnableCache(true),
	}

	if registry.IsOCI(c.repo) {
		ociAuth, err := c.ociAuthProvider.FindAuthEntry(ctx, c.repo)
		if err != nil {
			return nil, nil, err
		}
		if ociAuth != nil {
			var authOpts []registry.ClientOption
			authOpts, cleanup, err = ociAuth.BuildHelmRegistryOptions(ctx, u.Host)
			if err != nil {
				return nil, nil, err
			}
			opts = append(opts, authOpts...)
		}
	} else {
		opts = append(opts, registry.ClientOptCredentialsFile(settings.RegistryConfig))
	}

	// Create a new registry client
	registryClient, err := registry.NewClient(opts...)
	if err != nil {
		cleanup()
		return nil, nil, err
	}
	return registryClient, cleanup, nil
}

func (c *Chart) pullFromRegistry(ctx context.Context, version string, tmpPullDir string, chartDir string) error {
	u, err := url.Parse(c.repo)
	if err != nil {
		return err
	}
	settings := cli.New()
	registryClient, cleanup, err := c.newRegistryClient(ctx, settings)
	if err != nil {
		return err
	}
	defer cleanup()

	cfg, err := buildHelmConfig(nil, registryClient)
	if err != nil {
		return err
	}

	a := action.NewPullWithOpts(action.WithConfig(cfg))
	a.Settings = settings
	a.Untar = true
	a.DestDir = tmpPullDir
	a.Version = version

	if c.credentialsId != "" {
		if registry.IsOCI(c.repo) {
			return fmt.Errorf("OCI charts can currently only be authenticated via registry login and environment variables but not via cli arguments")
		}
	}

	if !registry.IsOCI(c.repo) {
		helmCreds, cf, err := c.helmAuthProvider.FindAuthEntry(ctx, *u, c.credentialsId)
		if err != nil {
			return err
		}
		defer cf()

		if helmCreds != nil {
			a.Username = helmCreds.Username
			a.Password = helmCreds.Password
			a.CertFile = helmCreds.CertFile
			a.CaFile = helmCreds.CAFile
			a.KeyFile = helmCreds.KeyFile
			a.InsecureSkipTLSverify = helmCreds.InsecureSkipTLSverify
			a.PassCredentialsAll = helmCreds.PassCredentialsAll
		}
	}

	var out string
	if registry.IsOCI(c.repo) {
		out, err = a.Run(c.repo)
	} else {
		a.RepoURL = c.repo
		out, err = a.Run(c.chartName)
	}
	if out != "" {
		status.Infof(ctx, "Message from Helm:\n%s", out)
	}
	if err != nil {
		return err
	}
	// move chart
	des, err := os.ReadDir(filepath.Join(tmpPullDir, c.chartName))
	if err != nil {
		return err
	}
	for _, de := range des {
		err = os.Rename(filepath.Join(tmpPullDir, c.chartName, de.Name()), filepath.Join(chartDir, de.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *Chart) pullFromGitRepository(ctx context.Context, chartDir string) error {
	m, err := c.gitRp.GetEntry(c.git.Url.String())
	if err != nil {
		return err
	}
	err = m.Update()
	if err != nil {
		return err
	}
	cd, gitInfo, err := m.GetClonedDir(c.git.Ref)
	if err != nil {
		return err
	}
	des, err := os.ReadDir(filepath.Join(cd, c.git.SubDir))
	if err != nil {
		return err
	}

	for _, de := range des {
		err = os.Rename(filepath.Join(cd, c.git.SubDir, de.Name()), filepath.Join(chartDir, de.Name()))
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}

	out, err := json.Marshal(gitInfo)
	if err != nil {
		return err
	}
	err = os.WriteFile(filepath.Join(chartDir, ".git-info"), out, 0o755)
	if err != nil {
		return err
	}
	return nil
}

func (c *Chart) PullToTmp(ctx context.Context, version string) (*PulledChart, error) {
	if c.IsLocalChart() {
		return nil, fmt.Errorf("can not pull local charts")
	}

	tmpPullDir, err := os.MkdirTemp(utils.GetTmpBaseDir(ctx), c.chartName+"-pull-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmpPullDir)

	chartDir, err := os.MkdirTemp(utils.GetTmpBaseDir(ctx), c.chartName+"-pulled-")
	if err != nil {
		return nil, err
	}
	if c.IsRegistryChart() {
		err = c.pullFromRegistry(ctx, version, tmpPullDir, chartDir)
	} else if c.IsGitRepositoryChart() {
		err = c.pullFromGitRepository(ctx, chartDir)
	} else {
		return nil, fmt.Errorf("unknown type of helm chart source")
	}
	if err != nil {
		return nil, err
	}

	return NewPulledChart(c, version, chartDir, true), nil
}

func (c *Chart) Pull(ctx context.Context, pc *PulledChart) error {
	if c.IsLocalChart() {
		return fmt.Errorf("can not pull local charts")
	}

	newPulled, err := c.PullToTmp(ctx, pc.version)
	if err != nil {
		return err
	}
	defer os.RemoveAll(newPulled.dir)

	err = os.RemoveAll(pc.dir)
	if err != nil {
		return err
	}

	_ = os.MkdirAll(filepath.Dir(pc.dir), 0o755)

	err = cp.Copy(newPulled.dir, pc.dir)
	if err != nil {
		return err
	}

	return nil
}

func (c *Chart) doPullCached(ctx context.Context, version string) (*PulledChart, *lockedfile.File, error) {
	baseDir := filepath.Join(utils.GetCacheDir(ctx), "helm-charts")
	cacheDir, err := c.BuildVersionedPulledChartDir(baseDir, version)
	_ = os.MkdirAll(cacheDir, 0o755)

	lock, err := lockedfile.Create(cacheDir + ".lock")
	if err != nil {
		return nil, nil, err
	}

	cached := NewPulledChart(c, version, cacheDir, true)
	needsPull, _, _, err := cached.CheckNeedsPull()
	if err != nil {
		_ = lock.Close()
		return nil, nil, err
	}
	if !needsPull {
		return cached, lock, nil
	}

	err = c.Pull(ctx, cached)
	if err != nil {
		_ = lock.Close()
		return nil, nil, err
	}

	return cached, lock, nil
}

func (c *Chart) PullCached(ctx context.Context, version string) (*PulledChart, error) {
	if c.IsLocalChart() {
		return nil, fmt.Errorf("can not pull local charts")
	}

	pc, lock, err := c.doPullCached(ctx, version)
	if err != nil {
		return nil, err
	}
	_ = lock.Close()
	return pc, nil
}

func (c *Chart) PullInProject(ctx context.Context, baseDir string, version string) (*PulledChart, error) {
	if c.IsLocalChart() {
		return nil, fmt.Errorf("can not pull local charts")
	}

	cachePc, lock, err := c.doPullCached(ctx, version)
	if err != nil {
		return nil, err
	}
	defer lock.Close()

	pc, err := c.GetPrePulledChart(baseDir, version)
	if err != nil {
		return nil, err
	}

	err = os.RemoveAll(pc.dir)
	if err != nil {
		return nil, err
	}

	err = cp.Copy(cachePc.dir, pc.dir)
	if err != nil {
		return nil, err
	}

	return pc, nil
}

func (c *Chart) GetPrePulledChart(baseDir string, version string) (*PulledChart, error) {
	if c.IsLocalChart() {
		return nil, fmt.Errorf("can not pull local charts")
	}
	chartDir, err := c.BuildVersionedPulledChartDir(baseDir, version)
	if err != nil {
		return nil, err
	}
	return NewPulledChart(c, version, chartDir, false), nil
}

func (c *Chart) QueryVersions(ctx context.Context) error {
	if c.IsLocalChart() {
		return fmt.Errorf("can not query versions for local charts")
	}
	if c.IsRegistryChart() {
		if registry.IsOCI(c.repo) {
			return c.queryVersionsOci(ctx)
		} else {
			return c.queryVersionsHelmRepo(ctx)
		}
	}
	if c.IsGitRepositoryChart() {
		return c.queryVersionsGitRepo(ctx)
	}
	return fmt.Errorf("chart type is not supported! Please use a local, registry or repository chart.")
}

func (c *Chart) queryVersionsOci(ctx context.Context) error {
	var clientOpts []crane.Option
	clientOpts = append(clientOpts, crane.WithContext(ctx))
	if c.ociAuthProvider != nil {
		auth, err := c.ociAuthProvider.FindAuthEntry(ctx, c.repo)
		if err != nil {
			return err
		}
		authOpts, err := auth.BuildCraneOptions()
		if err != nil {
			return nil
		}
		clientOpts = append(clientOpts, authOpts...)
	}

	imageName := strings.TrimPrefix(c.repo, "oci://")
	tags, err := crane.ListTags(imageName, clientOpts...)
	if err != nil {
		return err
	}

	c.versions = tags

	return nil
}

func (c *Chart) queryVersionsGitRepo(ctx context.Context) error {
	ref, refType, err := c.GetGitRef()
	if err != nil {
		return err
	}
	if refType == "branch" {
		return nil
	}
	if refType == "commit" {
		// Could be implemented at some point
		return nil
	}
	if refType == "tag" {
		if IsSemantic(ref) {
			m, err := c.gitRp.GetEntry(c.git.Url.String())
			if err != nil {
				return err
			}
			m.Update()
			refs := m.GetRepoInfo().RemoteRefs
			var semanticTags []string
			for ref, _ := range refs {
				after, found := strings.CutPrefix(ref, "refs/tags/")
				if found {
					semanticTags = append(semanticTags, after)
				}
			}
			c.versions = semanticTags
			return nil
		}
	}
	return nil
}

func (c *Chart) queryVersionsHelmRepo(ctx context.Context) error {
	settings := cli.New()

	u, err := url.Parse(c.repo)
	if err != nil {
		return err
	}

	e, cf, err := c.helmAuthProvider.FindAuthEntry(ctx, *u, c.credentialsId)
	if err != nil {
		return err
	}
	defer cf()
	if e == nil {
		if c.credentialsId != "" {
			return fmt.Errorf("no credentials found for Chart %s", c.chartName)
		}
		e = &repo.Entry{
			URL: c.repo,
		}
	}

	r, err := repo.NewChartRepository(e, getter.All(settings))
	if err != nil {
		return err
	}

	r.CachePath, err = os.MkdirTemp(utils.GetTmpBaseDir(ctx), "helm-check-update-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(r.CachePath)

	indexFile, err := r.DownloadIndexFile()
	if err != nil {
		return err
	}

	index, err := repo.LoadIndexFile(indexFile)
	if err != nil {
		return err
	}

	indexEntry, ok := index.Entries[c.chartName]
	if !ok || len(indexEntry) == 0 {
		return fmt.Errorf("helm chart %s not found in Repo index", c.chartName)
	}

	c.versions = make([]string, 0, indexEntry.Len())
	for _, x := range indexEntry {
		c.versions = append(c.versions, x.Version)
	}

	return nil
}

func (c *Chart) GetLatestVersion(constraints *string) (string, error) {
	if len(c.versions) == 0 {
		return "", fmt.Errorf("no versions found or queried: %s", c.GetChartName())
	}
	var err error
	var updateConstraints *semver.Constraints
	if constraints != nil {
		updateConstraints, err = semver.NewConstraint(*constraints)
		if err != nil {
			return "", fmt.Errorf("invalid constraints '%s': %w", *constraints, err)
		}
	}

	var versions semver.Collection
	for _, x := range c.versions {
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
		if constraints == nil {
			return "", fmt.Errorf("no version found")
		} else {
			return "", fmt.Errorf("no version found that satisfies constraints '%s'", *constraints)
		}
	}

	sort.Stable(versions)
	latestVersion := versions[len(versions)-1].Original()
	return latestVersion, nil
}
