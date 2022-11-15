package e2e

import (
	"fmt"
	"github.com/go-git/go-git/v5"
	"github.com/huandu/xstrings"
	"github.com/imdario/mergo"
	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type testProject struct {
	t        *testing.T
	extraEnv []string

	mergedKubeconfig string

	gitServer *test_utils.GitServer
}

func (p *testProject) init(t *testing.T, k *test_utils.EnvTestCluster) {
	p.t = t
	p.gitServer = test_utils.NewGitServer(t)
	p.gitServer.GitInit(p.getKluctlProjectRepo())

	p.updateKluctlYaml(func(o *uo.UnstructuredObject) error {
		return nil
	})
	p.updateDeploymentYaml(".", func(c *uo.UnstructuredObject) error {
		return nil
	})

	tmpFile, err := os.CreateTemp("", p.testSlug()+"-kubeconfig-")
	if err != nil {
		t.Fatal(err)
	}
	_ = tmpFile.Close()
	p.mergedKubeconfig = tmpFile.Name()
	t.Cleanup(func() {
		os.Remove(p.mergedKubeconfig)
	})
	p.mergeKubeconfig(k)
}

func (p *testProject) testSlug() string {
	n := p.t.Name()
	n = xstrings.ToKebabCase(n)
	return n
}

func (p *testProject) mergeKubeconfig(k *test_utils.EnvTestCluster) {
	p.updateMergedKubeconfig(func(config *clientcmdapi.Config) {
		nkcfg, err := clientcmd.Load(k.Kubeconfig)
		if err != nil {
			p.t.Fatal(err)
		}

		err = mergo.Merge(config, nkcfg)
		if err != nil {
			p.t.Fatal(err)
		}
	})
}

func (p *testProject) updateMergedKubeconfig(cb func(config *clientcmdapi.Config)) {
	mkcfg, err := clientcmd.LoadFromFile(p.mergedKubeconfig)
	if err != nil {
		p.t.Fatal(err)
	}

	cb(mkcfg)

	err = clientcmd.WriteToFile(*mkcfg, p.mergedKubeconfig)
	if err != nil {
		p.t.Fatal(err)
	}
}

func (p *testProject) updateKluctlYaml(update func(o *uo.UnstructuredObject) error) {
	p.updateYaml(".kluctl.yml", update, "")
}

func (p *testProject) updateDeploymentYaml(dir string, update func(o *uo.UnstructuredObject) error) {
	p.updateYaml(filepath.Join(dir, "deployment.yml"), func(o *uo.UnstructuredObject) error {
		if dir == "." {
			o.SetNestedField(p.testSlug(), "commonLabels", "project_name")
		}
		return update(o)
	}, "")
}

func (p *testProject) updateYaml(path string, update func(o *uo.UnstructuredObject) error, message string) {
	p.gitServer.UpdateYaml(p.getKluctlProjectRepo(), path, func(o *uo.UnstructuredObject) error {
		return update(o)
	}, message)
}

func (p *testProject) updateFile(path string, update func(f string) (string, error), message string) {
	p.gitServer.UpdateFile(p.getKluctlProjectRepo(), path, update, message)
}

func (p *testProject) getDeploymentYaml(dir string) *uo.UnstructuredObject {
	o, err := uo.FromFile(filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), dir, "deployment.yml"))
	if err != nil {
		p.t.Fatal(err)
	}
	return o
}

func (p *testProject) listDeploymentItemPathes(dir string, fullPath bool) []string {
	var ret []string
	o := p.getDeploymentYaml(dir)
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
			ret = append(ret, p.listDeploymentItemPathes(filepath.Join(dir, pth), fullPath)...)
		}
	}
	return ret
}

func (p *testProject) updateKustomizeDeployment(dir string, update func(o *uo.UnstructuredObject, wt *git.Worktree) error) {
	wt := p.gitServer.GetWorktree(p.getKluctlProjectRepo())

	pth := filepath.Join(dir, "kustomization.yml")
	p.updateYaml(pth, func(o *uo.UnstructuredObject) error {
		return update(o, wt)
	}, fmt.Sprintf("Update kustomization.yml for %s", dir))
}

func (p *testProject) updateTarget(name string, cb func(target *uo.UnstructuredObject)) {
	p.updateNamedListItem(uo.KeyPath{"targets"}, name, cb)
}

func (p *testProject) updateSecretSet(name string, cb func(secretSet *uo.UnstructuredObject)) {
	p.updateNamedListItem(uo.KeyPath{"secretsConfig", "secretSets"}, name, cb)
}

func (p *testProject) updateNamedListItem(path uo.KeyPath, name string, cb func(item *uo.UnstructuredObject)) {
	if cb == nil {
		cb = func(target *uo.UnstructuredObject) {}
	}

	p.updateKluctlYaml(func(o *uo.UnstructuredObject) error {
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

func (p *testProject) updateDeploymentItems(dir string, update func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject) {
	p.updateDeploymentYaml(dir, func(o *uo.UnstructuredObject) error {
		items, _, _ := o.GetNestedObjectList("deployments")
		items = update(items)
		return o.SetNestedField(items, "deployments")
	})
}

func (p *testProject) addDeploymentItem(dir string, item *uo.UnstructuredObject) {
	p.updateDeploymentItems(dir, func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
		for _, x := range items {
			if reflect.DeepEqual(x, item) {
				return items
			}
		}
		items = append(items, item)
		return items
	})
}

func (p *testProject) addDeploymentInclude(dir string, includePath string, tags []string) {
	n := uo.FromMap(map[string]interface{}{
		"include": includePath,
	})
	if len(tags) != 0 {
		n.SetNestedField(tags, "tags")
	}
	p.addDeploymentItem(dir, n)
}

func (p *testProject) addDeploymentIncludes(dir string) {
	var pp []string
	for _, x := range strings.Split(dir, "/") {
		if x != "." {
			p.addDeploymentInclude(filepath.Join(pp...), x, nil)
		}
		pp = append(pp, x)
	}
}

func (p *testProject) addKustomizeDeployment(dir string, resources []kustomizeResource, tags []string) {
	deploymentDir := filepath.Dir(dir)
	if deploymentDir != "" {
		p.addDeploymentIncludes(deploymentDir)
	}

	absKustomizeDir := filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), dir)

	err := os.MkdirAll(absKustomizeDir, 0o700)
	if err != nil {
		p.t.Fatal(err)
	}

	p.updateKustomizeDeployment(dir, func(o *uo.UnstructuredObject, wt *git.Worktree) error {
		o.SetNestedField("kustomize.config.k8s.io/v1beta1", "apiVersion")
		o.SetNestedField("Kustomization", "kind")
		return nil
	})

	p.addKustomizeResources(dir, resources)
	p.updateDeploymentYaml(deploymentDir, func(o *uo.UnstructuredObject) error {
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

func (p *testProject) convertInterfaceToList(x interface{}) []interface{} {
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

type kustomizeResource struct {
	name     string
	fileName string
	content  interface{}
}

func (p *testProject) addKustomizeResources(dir string, resources []kustomizeResource) {
	p.updateKustomizeDeployment(dir, func(o *uo.UnstructuredObject, wt *git.Worktree) error {
		l, _, _ := o.GetNestedList("resources")
		for _, r := range resources {
			l = append(l, r.name)
			fileName := r.fileName
			if fileName == "" {
				fileName = r.name
			}
			if r.content != nil {
				x := p.convertInterfaceToList(r.content)
				err := yaml.WriteYamlAllFile(filepath.Join(p.gitServer.LocalRepoDir(p.getKluctlProjectRepo()), dir, fileName), x)
				if err != nil {
					return err
				}
				_, err = wt.Add(filepath.Join(dir, fileName))
				if err != nil {
					return err
				}
			}
		}
		o.SetNestedField(l, "resources")
		return nil
	})
}

func (p *testProject) deleteKustomizeDeployment(dir string) {
	deploymentDir := filepath.Dir(dir)
	p.updateDeploymentItems(deploymentDir, func(items []*uo.UnstructuredObject) []*uo.UnstructuredObject {
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

func (p *testProject) getKluctlProjectRepo() string {
	return "kluctl-project"
}

func (p *testProject) Kluctl(argsIn ...string) (string, string, error) {
	var args []string
	args = append(args, argsIn...)
	args = append(args, "--no-update-check")

	cwd := p.gitServer.LocalRepoDir(p.getKluctlProjectRepo())

	args = append(args, "--debug")

	env := os.Environ()
	env = append(env, p.extraEnv...)
	env = append(env, fmt.Sprintf("KUBECONFIG=%s", p.mergedKubeconfig))

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

func (p *testProject) KluctlMust(argsIn ...string) (string, string) {
	stdout, stderr, err := p.Kluctl(argsIn...)
	if err != nil {
		p.t.Logf(stderr)
		p.t.Fatal(fmt.Errorf("kluctl failed: %w", err))
	}
	return stdout, stderr
}
