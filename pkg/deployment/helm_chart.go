package deployment

import (
	"fmt"
	"github.com/codablock/kluctl/pkg/k8s"
	"github.com/codablock/kluctl/pkg/types"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/yaml"
	goversion "github.com/hashicorp/go-version"
	"io/ioutil"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strings"
)

type helmChart struct {
	configFile string
	config     *types.HelmChartConfig
}

func NewHelmChart(configFile string) (*helmChart, error) {
	var config types.HelmChartConfig
	err := yaml.ReadYamlFile(configFile, &config)
	if err != nil {
		return nil, err
	}

	hc := &helmChart{
		configFile: configFile,
		config:     &config,
	}
	return hc, nil
}

func (c *helmChart) withRepoContext(cb func(repoName string) error) error {
	needRepo := false
	repoName := "stable"

	if c.config.Repo != nil && *c.config.Repo != "stable" {
		needRepo = true
		repoName = fmt.Sprintf("kluctl-%s", utils.Sha256String(*c.config.Repo))[:16]
	}

	if needRepo {
		_, err := c.doHelm([]string{"repo", "remove", repoName}, true)
		if err != nil {
			return err
		}
		_, err = c.doHelm([]string{"repo", "add", repoName, *c.config.Repo}, false)
		if err != nil {
			return err
		}
		defer func() {
			_, _ = c.doHelm([]string{"repo", "remove", repoName}, true)
		}()
	} else {
		_, err := c.doHelm([]string{"repo", "update"}, false)
		if err != nil {
			return err
		}
	}
	return cb(repoName)
}

func (c *helmChart) GetChartName() (string, error) {
	if c.config.Repo != nil && strings.HasPrefix(*c.config.Repo, "oci://") {
		s := strings.Split(*c.config.Repo, "/")
		chartName := s[len(s)-1]
		if m, _ := regexp.MatchString(`[a-zA-Z_-]+`, chartName); !m {
			return "", fmt.Errorf("invalid oci chart url: %s", *c.config.Repo)
		}
		return chartName, nil
	}
	if c.config.ChartName == nil {
		return "", fmt.Errorf("chartName is missing in helm-chart.yml")
	}
	return *c.config.ChartName, nil
}

func (c *helmChart) Pull() error {
	chartName, err := c.GetChartName()
	if err != nil {
		return err
	}

	dir := path.Dir(c.configFile)
	targetDir := path.Join(dir, "charts")
	rmDir := path.Join(targetDir, chartName)
	_ = os.RemoveAll(rmDir)

	var args []string
	if c.config.Repo != nil && strings.HasPrefix(*c.config.Repo, "oci://") {
		args = []string{"pull", *c.config.Repo, "--destination", targetDir, "--untar"}
		if c.config.ChartVersion != nil {
			args = append(args, "--version")
			args = append(args, *c.config.ChartVersion)
		}
		_, err = c.doHelm(args, false)
		return err
	} else {
		return c.withRepoContext(func(repoName string) error {
			args = []string{"pull", fmt.Sprintf("%s/%s", repoName, chartName), "--destination", targetDir, "--untar"}
			if c.config.ChartVersion != nil {
				args = append(args, "--version", *c.config.ChartVersion)
			}
			_, err = c.doHelm(args, false)
			return err
		})
	}
}

type VersionSlice []string

func (x VersionSlice) Less(i, j int) bool {
	v1, err1 := goversion.NewVersion(x[i])
	v2, err2 := goversion.NewVersion(x[j])
	if err1 != nil {
		v1, _ = goversion.NewVersion("0")
	}
	if err2 != nil {
		v2, _ = goversion.NewVersion("0")
	}
	return v1.LessThan(v2)
}
func (x VersionSlice) Len() int      { return len(x) }
func (x VersionSlice) Swap(i, j int) { x[i], x[j] = x[j], x[i] }

func (c *helmChart) CheckUpdate() (string, error) {
	if c.config.Repo != nil && strings.HasPrefix(*c.config.Repo, "oci://") {
		return "", nil
	}
	chartName, err := c.GetChartName()
	if err != nil {
		return "", err
	}
	var latestVersion string
	err = c.withRepoContext(func(repoName string) error {
		chartName := fmt.Sprintf("%s/%s", repoName, chartName)
		args := []string{"search", "repo", chartName, "-oyaml", "-l"}
		stdout, err := c.doHelm(args, false)
		if err != nil {
			return err
		}
		// ensure we didn't get partial matches
		var lm []map[string]string
		var ls VersionSlice
		err = yaml.ReadYamlBytes(stdout, &lm)
		if err != nil {
			return err
		}
		for _, x := range lm {
			if n, ok := x["name"]; ok && n == chartName {
				if v, ok := x["version"]; ok {
					ls = append(ls, v)
				}
			}
		}
		if len(ls) == 0 {
			return fmt.Errorf("helm chart %s not found in repository", chartName)
		}
		sort.Sort(ls)
		latestVersion = ls[len(ls)-1]
		if c.config.ChartVersion != nil && latestVersion == *c.config.ChartVersion {
			return nil
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return latestVersion, nil
}

func (c *helmChart) Render(k *k8s.K8sCluster) error {
	chartName, err := c.GetChartName()
	if err != nil {
		return err
	}
	dir := path.Dir(c.configFile)
	chartDir := path.Join(dir, "charts", chartName)
	valuesPath := path.Join(dir, "helm-values.yml")
	outputPath := path.Join(dir, c.config.Output)

	args := []string{"template", c.config.ReleaseName, chartDir}

	namespace := "default"
	if c.config.Namespace != nil {
		namespace = *c.config.Namespace
	}

	if utils.Exists(valuesPath) {
		args = append(args, "-f", valuesPath)
	}
	args = append(args, "-n", namespace)

	if c.config.SkipCRDs != nil && *c.config.SkipCRDs {
		args = append(args, "--skip-crds")
	} else {
		args = append(args, "--include-crds")
	}
	args = append(args, "--skip-tests")

	gvs, err := k.GetAllGroupVersions()
	if err != nil {
		return err
	}
	for _, gv := range gvs {
		args = append(args, fmt.Sprintf("--api-versions=%s", gv))
	}
	args = append(args, fmt.Sprintf("--kube-version=%s", k.ServerVersion.String()))

	rendered, err := c.doHelm(args, false)
	if err != nil {
		return err
	}

	parsed, err := yaml.ReadYamlAllBytes(rendered)

	for _, o := range parsed {
		m, ok := o.(map[string]interface{})
		if !ok {
			return fmt.Errorf("object is not a map")
		}

		// "helm install" will deploy resources to the given namespace automatically, but "helm template" does not
		// add the necessary namespace in the rendered resources
		_, found, _ := unstructured.NestedString(m, "metadata", "namespace")
		if !found && k.IsNamespaced((&unstructured.Unstructured{Object: m}).GroupVersionKind()) {
			_ = unstructured.SetNestedField(m, namespace, "metadata", "namespace")
		}
	}
	rendered, err = yaml.WriteYamlAllBytes(parsed)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(outputPath, rendered, 0o666)
	if err != nil {
		return err
	}
	return nil
}

func (c *helmChart) doHelm(args []string, ignoreStdErr bool) ([]byte, error) {
	cmd := exec.Command("helm", args...)
	cmd.Env = append(os.Environ(), "HELM_EXPERIMENTAL_OCI=true")

	if ignoreStdErr {
		stderrPipe, err := cmd.StderrPipe()
		if err != nil {
			return nil, err
		}
		go func() {
			buf := make([]byte, 1024)
			for true {
				n, err := stderrPipe.Read(buf)
				if err != nil || n == 0 {
					return
				}
			}
		}()
	} else {
		cmd.Stderr = os.Stderr
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		return nil, err
	}
	defer cmd.Process.Kill()

	stdout, err := ioutil.ReadAll(stdoutPipe)
	if err != nil {
		return nil, err
	}
	ps, err := cmd.Process.Wait()
	if err != nil {
		return nil, err
	}
	if ps.ExitCode() != 0 {
		return nil, fmt.Errorf("helm returned non-zero exit code %d", ps.ExitCode())
	}
	return stdout, nil
}
