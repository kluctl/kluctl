package test_utils

import (
	"context"
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/huandu/xstrings"
	"github.com/jinzhu/copier"
	git2 "github.com/kluctl/kluctl/v2/pkg/git"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	registry2 "helm.sh/helm/v3/pkg/registry"
	"k8s.io/apimachinery/pkg/util/rand"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type TestProject struct {
	t *testing.T

	extraEnv   []string
	useProcess bool
	bare       bool

	gitServer   *git2.TestGitServer
	gitRepoName string
	gitSubDir   string
}

type TestProjectOption func(p *TestProject)

func WithUseProcess(useProcess bool) TestProjectOption {
	return func(p *TestProject) {
		p.useProcess = useProcess
	}
}

func WithGitServer(s *git2.TestGitServer) TestProjectOption {
	return func(p *TestProject) {
		p.gitServer = s
	}
}

func WithRepoName(n string) TestProjectOption {
	return func(p *TestProject) {
		p.gitRepoName = n
	}
}

func WithGitSubDir(subDir string) TestProjectOption {
	return func(p *TestProject) {
		p.gitSubDir = subDir
	}
}

func WithBareProject() TestProjectOption {
	return func(p *TestProject) {
		p.bare = true
	}
}

func NewTestProject(t *testing.T, opts ...TestProjectOption) *TestProject {
	p := &TestProject{
		t:           t,
		gitRepoName: "kluctl-project",
	}

	for _, o := range opts {
		o(p)
	}

	if p.gitServer == nil {
		p.gitServer = git2.NewTestGitServer(t)
	}
	p.gitServer.GitInit(p.gitRepoName)

	if !p.bare {
		p.UpdateKluctlYaml(func(o *uo.UnstructuredObject) error {
			_ = o.SetNestedField(fmt.Sprintf("%s-{{ target.name or 'no-name' }}", rand.String(16)), "discriminator")
			return nil
		})
		p.UpdateDeploymentYaml(".", func(c *uo.UnstructuredObject) error {
			return nil
		})
	}
	return p
}

func (p *TestProject) GitServer() *git2.TestGitServer {
	return p.gitServer
}

func (p *TestProject) TestSlug() string {
	n := p.t.Name()
	n = xstrings.ToKebabCase(n)
	n = strings.ReplaceAll(n, "/", "-")
	return n
}

func (p *TestProject) AddExtraEnv(e string) {
	p.extraEnv = append(p.extraEnv, e)
}

func (p *TestProject) UpdateKluctlYaml(update func(o *uo.UnstructuredObject) error) {
	p.UpdateYaml(".kluctl.yml", update, "")
}

func (p *TestProject) UpdateDeploymentYaml(dir string, update func(o *uo.UnstructuredObject) error) {
	p.UpdateYaml(filepath.Join(dir, "deployment.yml"), func(o *uo.UnstructuredObject) error {
		if dir == "." {
			o.SetNestedField(p.TestSlug(), "commonLabels", "project_name")
		}
		return update(o)
	}, "")
}

func (p *TestProject) UpdateYaml(pth string, update func(o *uo.UnstructuredObject) error, message string) {
	p.gitServer.UpdateYaml(p.gitRepoName, path.Join(p.gitSubDir, pth), func(o map[string]any) error {
		u := uo.FromMap(o)
		err := update(u)
		if err != nil {
			return err
		}
		_ = copier.CopyWithOption(&o, &u.Object, copier.Option{DeepCopy: true})
		return nil
	}, message)
}

func (p *TestProject) UpdateFile(pth string, update func(f string) (string, error), message string) {
	p.gitServer.UpdateFile(p.gitRepoName, path.Join(p.gitSubDir, pth), update, message)
}

func (p *TestProject) GetYaml(path string) *uo.UnstructuredObject {
	o, err := uo.FromFile(filepath.Join(p.LocalProjectDir(), path))
	if err != nil {
		p.t.Fatal(err)
	}
	return o
}

func (p *TestProject) GetDeploymentYaml(dir string) *uo.UnstructuredObject {
	return p.GetYaml(filepath.Join(dir, "deployment.yml"))
}

func (p *TestProject) ListDeploymentItemPathes(dir string, fullPath bool) []string {
	var ret []string
	o := p.GetDeploymentYaml(dir)
	l, _, err := o.GetNestedObjectList("deployments")
	if err != nil {
		p.t.Fatal(err)
	}
	for _, x := range l {
		pth, ok, _ := x.GetNestedString("path")
		if ok {
			x := pth
			if fullPath {
				x = filepath.Join(dir, pth)
			}
			ret = append(ret, x)
		}
		pth, ok, _ = x.GetNestedString("include")
		if ok {
			ret = append(ret, p.ListDeploymentItemPathes(filepath.Join(dir, pth), fullPath)...)
		}
	}
	return ret
}

func (p *TestProject) UpdateKustomizeDeployment(dir string, update func(o *uo.UnstructuredObject, wt *git.Worktree) error) {
	wt := p.gitServer.GetWorktree(p.gitRepoName)

	pth := filepath.Join(dir, "kustomization.yml")
	p.UpdateYaml(pth, func(o *uo.UnstructuredObject) error {
		return update(o, wt)
	}, fmt.Sprintf("Update kustomization.yml for %s", dir))
}

func (p *TestProject) UpdateTarget(name string, cb func(target *uo.UnstructuredObject)) {
	p.UpdateNamedListItem(uo.KeyPath{"targets"}, name, cb)
}

func (p *TestProject) UpdateSecretSet(name string, cb func(secretSet *uo.UnstructuredObject)) {
	p.UpdateNamedListItem(uo.KeyPath{"secretsConfig", "secretSets"}, name, cb)
}

func (p *TestProject) UpdateNamedListItem(path uo.KeyPath, name string, cb func(item *uo.UnstructuredObject)) {
	if cb == nil {
		cb = func(target *uo.UnstructuredObject) {}
	}

	p.UpdateKluctlYaml(func(o *uo.UnstructuredObject) error {
		l, _, _ := o.GetNestedObjectList(path...)
		var newList []*uo.UnstructuredObject
		found := false
		for _, item := range l {
			n, _, _ := item.GetNestedString("name")
			if n == name {
				cb(item)
				found = true
			}
			newList = append(newList, item)
		}
		if !found {
			n := uo.FromMap(map[string]interface{}{
				"name": name,
			})
			cb(n)
			newList = append(newList, n)
		}

		_ = o.SetNestedObjectList(newList, path...)
		return nil
	})
}

func (p *TestProject) UpdateDeploymentItems(dir string, update func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject) {
	p.UpdateDeploymentYaml(dir, func(o *uo.UnstructuredObject) error {
		items, _, _ := o.GetNestedObjectList("deployments")
		items = update(items)
		return o.SetNestedField(items, "deployments")
	})
}

func (p *TestProject) AddDeploymentItem(dir string, item *uo.UnstructuredObject) {
	p.UpdateDeploymentItems(dir, func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
		for _, x := range items {
			if reflect.DeepEqual(x, item) {
				return items
			}
		}
		items = append(items, item)
		return items
	})
}

func (p *TestProject) AddDeploymentInclude(dir string, includePath string, tags []string) {
	n := uo.FromMap(map[string]interface{}{
		"include": includePath,
	})
	if len(tags) != 0 {
		n.SetNestedField(tags, "tags")
	}
	p.AddDeploymentItem(dir, n)
}

func (p *TestProject) AddDeploymentIncludes(dir string) {
	var pp []string
	for _, x := range strings.Split(dir, "/") {
		if x != "." {
			p.AddDeploymentInclude(filepath.Join(pp...), x, nil)
		}
		pp = append(pp, x)
	}
}

func (p *TestProject) AddKustomizeDeployment(dir string, resources []KustomizeResource, tags []string) {
	deploymentDir := filepath.Dir(dir)
	if deploymentDir != "" {
		p.AddDeploymentIncludes(deploymentDir)
	}

	absKustomizeDir := filepath.Join(p.LocalProjectDir(), dir)

	err := os.MkdirAll(absKustomizeDir, 0o700)
	if err != nil {
		p.t.Fatal(err)
	}

	p.UpdateKustomizeDeployment(dir, func(o *uo.UnstructuredObject, wt *git.Worktree) error {
		o.SetNestedField("kustomize.config.k8s.io/v1beta1", "apiVersion")
		o.SetNestedField("Kustomization", "kind")
		return nil
	})

	p.AddKustomizeResources(dir, resources)
	p.UpdateDeploymentYaml(deploymentDir, func(o *uo.UnstructuredObject) error {
		d, _, _ := o.GetNestedObjectList("deployments")
		n := uo.FromMap(map[string]interface{}{
			"path": filepath.Base(dir),
		})
		if len(tags) != 0 {
			n.SetNestedField(tags, "tags")
		}
		d = append(d, n)
		_ = o.SetNestedObjectList(d, "deployments")
		return nil
	})
}

func (p *TestProject) AddHelmDeployment(dir string, repoUrl string, chartName, version string, releaseName string, namespace string, values map[string]any) {
	localPath := ""
	if u, err := url.Parse(repoUrl); err != nil || u.Host == "" {
		localPath = repoUrl
		repoUrl = ""
	} else if registry2.IsOCI(repoUrl) {
		repoUrl += "/" + chartName
		chartName = ""
	}

	p.AddKustomizeDeployment(dir, []KustomizeResource{
		{Name: "helm-rendered.yaml"},
	}, nil)

	p.UpdateYaml(filepath.Join(dir, "helm-chart.yaml"), func(o *uo.UnstructuredObject) error {
		*o = *uo.FromMap(map[string]interface{}{
			"helmChart": map[string]any{
				"repo":         repoUrl,
				"path":         localPath,
				"chartVersion": version,
				"releaseName":  releaseName,
				"namespace":    namespace,
			},
		})
		if chartName != "" {
			_ = o.SetNestedField(chartName, "helmChart", "chartName")
		}
		return nil
	}, "")

	if values != nil {
		p.UpdateYaml(filepath.Join(dir, "helm-values.yaml"), func(o *uo.UnstructuredObject) error {
			*o = *uo.FromMap(values)
			return nil
		}, "")
	}
}

func (p *TestProject) convertInterfaceToList(x interface{}) []interface{} {
	var ret []interface{}
	if l, ok := x.([]interface{}); ok {
		return l
	}
	if l, ok := x.([]*uo.UnstructuredObject); ok {
		for _, y := range l {
			ret = append(ret, y)
		}
		return ret
	}
	if l, ok := x.([]map[string]interface{}); ok {
		for _, y := range l {
			ret = append(ret, y)
		}
		return ret
	}
	return []interface{}{x}
}

type KustomizeResource struct {
	Name     string
	FileName string
	Content  interface{}
}

func (p *TestProject) AddKustomizeResources(dir string, resources []KustomizeResource) {
	p.UpdateKustomizeDeployment(dir, func(o *uo.UnstructuredObject, wt *git.Worktree) error {
		l, _, _ := o.GetNestedList("resources")
		for _, r := range resources {
			l = append(l, r.Name)
			fileName := r.FileName
			if fileName == "" {
				fileName = r.Name
			}
			if r.Content != nil {
				x := p.convertInterfaceToList(r.Content)
				err := yaml.WriteYamlAllFile(filepath.Join(p.LocalProjectDir(), dir, fileName), x)
				if err != nil {
					return err
				}
				_, err = wt.Add(filepath.Join(path.Join(p.gitSubDir, dir), fileName))
				if err != nil {
					return err
				}
			}
		}
		o.SetNestedField(l, "resources")
		return nil
	})
}

func (p *TestProject) DeleteKustomizeDeployment(dir string) {
	deploymentDir := filepath.Dir(dir)
	p.UpdateDeploymentItems(deploymentDir, func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
		var newItems []*uo.UnstructuredObject
		for _, item := range items {
			pth, _, _ := item.GetNestedString("path")
			if pth == filepath.Base(dir) {
				continue
			}
			newItems = append(newItems, item)
		}
		return newItems
	})
}

func (p *TestProject) GitUrl() string {
	return p.gitServer.GitRepoUrl(p.gitRepoName)
}

func (p *TestProject) LocalRepoDir() string {
	return p.gitServer.LocalRepoDir(p.gitRepoName)
}

func (p *TestProject) LocalProjectDir() string {
	return path.Join(p.LocalRepoDir(), p.gitSubDir)
}

func (p *TestProject) GetGitRepo() *git.Repository {
	return p.gitServer.GetGitRepo(p.gitRepoName)
}

func (p *TestProject) KluctlProcess(argsIn ...string) (string, string, error) {
	var args []string
	args = append(args, argsIn...)
	args = append(args, "--no-update-check")

	cwd := p.LocalProjectDir()

	args = append(args, "--debug")

	env := os.Environ()
	env = append(env, p.extraEnv...)

	// this will cause the init() function from call_kluctl_hack.go to invoke the kluctl root command and then exit
	env = append(env, "CALL_KLUCTL=true")
	env = append(env, fmt.Sprintf("KLUCTL_BASE_TMP_DIR=%s", p.t.TempDir()))

	p.t.Logf("Runnning kluctl: %s", strings.Join(args, " "))

	testExe, err := os.Executable()
	if err != nil {
		panic(err)
	}

	cmd := exec.Command(testExe, args...)
	cmd.Dir = cwd
	cmd.Env = env

	stdout, stderr, err := runHelper(p.t, cmd)
	return stdout, stderr, err
}

func (p *TestProject) KluctlProcessMust(argsIn ...string) (string, string) {
	stdout, stderr, err := p.KluctlProcess(argsIn...)
	if err != nil {
		p.t.Logf(stderr)
		p.t.Fatal(fmt.Errorf("kluctl failed: %w", err))
	}
	return stdout, stderr
}

func (p *TestProject) KluctlExecute(argsIn ...string) (string, string, error) {
	if len(p.extraEnv) != 0 {
		p.t.Fatal("extraEnv is only supported in KluctlProcess(...)")
	}

	var args []string
	args = append(args, "--project-dir", p.LocalProjectDir())
	args = append(args, argsIn...)

	return KluctlExecute(p.t, context.Background(), args...)
}

func (p *TestProject) Kluctl(argsIn ...string) (string, string, error) {
	if p.useProcess {
		return p.KluctlProcess(argsIn...)
	} else {
		return p.KluctlExecute(argsIn...)
	}
}

func (p *TestProject) KluctlMust(argsIn ...string) (string, string) {
	stdout, stderr, err := p.Kluctl(argsIn...)
	if err != nil {
		p.t.Fatal(fmt.Errorf("kluctl failed: %w", err))
	}
	return stdout, stderr
}
