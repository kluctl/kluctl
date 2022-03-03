package e2e

import (
	"context"
	"fmt"
	http_server "github.com/codablock/kluctl/pkg/git/http-server"
	"github.com/codablock/kluctl/pkg/utils"
	"github.com/codablock/kluctl/pkg/utils/uo"
	"github.com/codablock/kluctl/pkg/yaml"
	"github.com/go-git/go-git/v5"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type testProject struct {
	t           *testing.T
	projectName string

	kluctlProjectExternal bool
	clustersExternal      bool
	deploymentExternal    bool
	sealedSecretsExternal bool

	localClusters      *string
	localDeployment    *string
	localSealedSecrets *string

	kubeconfigs []string

	baseDir string

	gitServer     *http_server.Server
	gitHttpServer *http.Server
	gitServerPort int
}

func (p *testProject) init(t *testing.T, projectName string) {
	p.t = t
	p.projectName = projectName
	baseDir, err := ioutil.TempDir(os.TempDir(), "kluctl-e2e-")
	if err != nil {
		p.t.Fatal(err)
	}
	p.baseDir = baseDir

	_ = os.MkdirAll(p.getKluctlProjectDir(), 0o700)
	_ = os.MkdirAll(filepath.Join(p.getClustersDir(), "clusters"), 0o700)
	_ = os.MkdirAll(filepath.Join(p.getSealedSecretsDir(), ".sealed-secrets"), 0o700)
	_ = os.MkdirAll(p.getDeploymentDir(), 0o700)

	p.initGitServer()

	p.gitInit(p.getKluctlProjectDir())
	if p.clustersExternal {
		p.gitInit(p.getClustersDir())
	}
	if p.deploymentExternal {
		p.gitInit(p.getDeploymentDir())
	}
	if p.sealedSecretsExternal {
		p.gitInit(p.getSealedSecretsDir())
	}

	p.updateKluctlYaml(func(o *uo.UnstructuredObject) error {
		if p.clustersExternal {
			o.SetNestedField(p.buildLocalGitUrl(p.getClustersDir()), "clusters", "project")
		}
		if p.deploymentExternal {
			o.SetNestedField(p.buildLocalGitUrl(p.getDeploymentDir()), "deployment", "project")
		}
		if p.sealedSecretsExternal {
			o.SetNestedField(p.buildLocalGitUrl(p.getSealedSecretsDir()), "sealedSecrets", "project")
		}
		return nil
	})
	p.updateDeploymentYaml(".", func(c *uo.UnstructuredObject) error {
		return nil
	})
}

func (p *testProject) initGitServer() {
	p.gitServer = http_server.New(p.baseDir)

	p.gitHttpServer = &http.Server{
		Addr:    "127.0.0.1:0",
		Handler: p.gitServer,
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}
	a := ln.Addr().(*net.TCPAddr)
	p.gitServerPort = a.Port

	go func() {
		_ = p.gitHttpServer.Serve(ln)
	}()
}

func (p *testProject) cleanup() {
	if p.gitHttpServer != nil {
		_ = p.gitHttpServer.Shutdown(context.Background())
		p.gitHttpServer = nil
		p.gitServer = nil
	}

	if p.baseDir == "" {
		return
	}
	_ = os.RemoveAll(p.baseDir)
	p.baseDir = ""
}

func (p *testProject) gitInit(dir string) {
	err := os.MkdirAll(dir, 0o700)
	if err != nil {
		p.t.Fatal(err)
	}

	r, err := git.PlainInit(dir, false)
	if err != nil {
		p.t.Fatal(err)
	}
	config, err := r.Config()
	if err != nil {
		p.t.Fatal(err)
	}
	wt, err := r.Worktree()
	if err != nil {
		p.t.Fatal(err)
	}

	config.User.Name = "Test User"
	config.User.Email = "no@mail.com"
	config.Author = config.User
	config.Committer = config.User
	err = r.SetConfig(config)
	if err != nil {
		p.t.Fatal(err)
	}
	err = utils.Touch(filepath.Join(dir, ".dummy"))
	if err != nil {
		p.t.Fatal(err)
	}
	_, err = wt.Add(".dummy")
	if err != nil {
		p.t.Fatal(err)
	}
	_, err = wt.Commit("initial", &git.CommitOptions{})
	if err != nil {
		p.t.Fatal(err)
	}
}

func (p *testProject) commitFiles(repo string, add []string, all bool, message string) {
	r, err := git.PlainOpen(repo)
	if err != nil {
		p.t.Fatal(err)
	}
	wt, err := r.Worktree()
	if err != nil {
		p.t.Fatal(err)
	}
	for _, a := range add {
		_, err = wt.Add(a)
		if err != nil {
			p.t.Fatal(err)
		}
	}
	_, err = wt.Commit(message, &git.CommitOptions{
		All: all,
	})
	if err != nil {
		p.t.Fatal(err)
	}
}

func (p *testProject) commitYaml(y *uo.UnstructuredObject, repo string, pth string, message string) {
	err := yaml.WriteYamlFile(filepath.Join(repo, pth), y)
	if err != nil {
		p.t.Fatal(err)
	}
	if message == "" {
		relPath, err := filepath.Rel(p.baseDir, repo)
		if err != nil {
			p.t.Fatal(err)
		}
		message = fmt.Sprintf("update %s", filepath.Join(relPath, pth))
	}
	p.commitFiles(repo, []string{pth}, false, message)
}

func (p *testProject) updateYaml(repo string, pth string, update func(o *uo.UnstructuredObject) error, message string) {
	if !strings.HasPrefix(repo, p.baseDir) {
		p.t.Fatal()
	}
	o := uo.New()
	if utils.Exists(filepath.Join(repo, pth)) {
		err := yaml.ReadYamlFile(filepath.Join(repo, pth), o)
		if err != nil {
			p.t.Fatal(err)
		}
	}
	orig := o.Clone()
	err := update(o)
	if err != nil {
		p.t.Fatal(err)
	}
	if reflect.DeepEqual(o, orig) {
		return
	}
	p.commitYaml(o, repo, pth, message)
}

func (p *testProject) updateKluctlYaml(update func(o *uo.UnstructuredObject) error) {
	p.updateYaml(p.getKluctlProjectDir(), ".kluctl.yml", update, "")
}

func (p *testProject) updateDeploymentYaml(dir string, update func(o *uo.UnstructuredObject) error) {
	p.updateYaml(p.getDeploymentDir(), filepath.Join(dir, "deployment.yml"), func(o *uo.UnstructuredObject) error {
		if dir == "." {
			o.SetNestedField(p.projectName, "commonLabels", "project_name")
			o.SetNestedField(p.projectName, "deleteByLabels", "project_name")
		}
		return update(o)
	}, "")
}

func (p *testProject) getDeploymentYaml(dir string) *uo.UnstructuredObject {
	o, err := uo.FromFile(filepath.Join(p.getDeploymentDir(), dir, "deployment.yml"))
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
	r, err := git.PlainOpen(p.getDeploymentDir())
	if err != nil {
		p.t.Fatal(err)
	}
	wt, err := r.Worktree()
	if err != nil {
		p.t.Fatal(err)
	}

	pth := filepath.Join(dir, "kustomization.yml")
	p.updateYaml(p.getDeploymentDir(), pth, func(o *uo.UnstructuredObject) error {
		return update(o, wt)
	}, fmt.Sprintf("Update kustomization.yml for %s", dir))
}

func (p *testProject) updateCluster(name string, context string, vars *uo.UnstructuredObject) {
	pth := filepath.Join("clusters", fmt.Sprintf("%s.yml", name))
	p.updateYaml(p.getClustersDir(), pth, func(o *uo.UnstructuredObject) error {
		o.Clear()
		o.SetNestedField(name, "cluster", "name")
		o.SetNestedField(context, "cluster", "context")
		if vars != nil {
			o.MergeChild("cluster", vars)
		}
		return nil
	}, fmt.Sprintf("add/update cluster %s", name))
}

func (p *testProject) updateKindCluster(k *KindCluster, vars *uo.UnstructuredObject) {
	if utils.FindStrInSlice(p.kubeconfigs, k.Kubeconfig()) == -1 {
		p.kubeconfigs = append(p.kubeconfigs, k.Kubeconfig())
	}
	context, err := k.Kubectl("config", "current-context")
	if err != nil {
		p.t.Fatal(err)
	}
	context = strings.TrimSpace(context)
	p.updateCluster(k.Name, context, vars)
}

func (p *testProject) updateTarget(name string, cluster string, args *uo.UnstructuredObject) {
	p.updateKluctlYaml(func(o *uo.UnstructuredObject) error {
		targets, _, _ := o.GetNestedObjectList("targets")
		var newTargets []*uo.UnstructuredObject
		for _, t := range targets {
			n, _, _ := t.GetNestedString("name")
			if n == name {
				continue
			}
			newTargets = append(newTargets, t)
		}
		n := uo.FromMap(map[string]interface{}{
			"name":    name,
			"cluster": cluster,
		})
		if args != nil {
			n.MergeChild("args", args)
		}

		newTargets = append(newTargets, n)
		_ = o.SetNestedObjectList(newTargets, "targets")
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

	absKustomizeDir := filepath.Join(p.getDeploymentDir(), dir)

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
	name    string
	content interface{}
}

func (p *testProject) addKustomizeResources(dir string, resources []kustomizeResource) {
	p.updateKustomizeDeployment(dir, func(o *uo.UnstructuredObject, wt *git.Worktree) error {
		l, _, _ := o.GetNestedList("resources")
		for _, r := range resources {
			l = append(l, r.name)
			x := p.convertInterfaceToList(r.content)
			err := yaml.WriteYamlAllFile(filepath.Join(p.getDeploymentDir(), dir, r.name), x)
			if err != nil {
				return err
			}
			_, err = wt.Add(filepath.Join(dir, r.name))
			if err != nil {
				return err
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

func (p *testProject) getKluctlProjectDir() string {
	return filepath.Join(p.baseDir, "kluctl-project")
}

func (p *testProject) getClustersDir() string {
	if p.clustersExternal {
		return filepath.Join(p.baseDir, "external-clusters")
	}
	return p.getKluctlProjectDir()
}

func (p *testProject) getDeploymentDir() string {
	if p.deploymentExternal {
		return filepath.Join(p.baseDir, "external-deployment")
	}
	return p.getKluctlProjectDir()
}

func (p *testProject) getSealedSecretsDir() string {
	if p.sealedSecretsExternal {
		return filepath.Join(p.baseDir, "external-sealed-secrets")
	}
	return p.getKluctlProjectDir()
}

const stdoutStartMarker = "========= stdout start ========="
const stdoutEndMarker = "========= stdout end ========="

func (p *testProject) Kluctl(argsIn ...string) (string, string, error) {
	var args []string
	args = append(args, argsIn...)
	args = append(args, "--no-update-check")

	cwd := ""
	if p.kluctlProjectExternal {
		args = append(args, "--project-url", p.buildLocalGitUrl(p.getKluctlProjectDir()))
	} else {
		cwd = p.getKluctlProjectDir()
	}

	if p.localClusters != nil {
		args = append(args, "--local-clusters", *p.localClusters)
	}
	if p.localDeployment != nil {
		args = append(args, "--local-deployment", *p.localDeployment)
	}
	if p.localSealedSecrets != nil {
		args = append(args, "--local-sealed-secrets", *p.localSealedSecrets)
	}

	sep := ":"
	if runtime.GOOS == "windows" {
		sep = ";"
		args = append(args, "-vdebug")
	}
	env := os.Environ()
	env = append(env, fmt.Sprintf("KUBECONFIG=%s", strings.Join(p.kubeconfigs, sep)))

	log.Infof("Runnning kluctl: %s", strings.Join(args, " "))

	stdout, stderr, err := runWrappedCmd("TestKluctlWrapper", cwd, env, args)
	return stdout, stderr, err
}

func (p *testProject) KluctlMust(argsIn ...string) (string, string) {
	stdout, stderr, err := p.Kluctl(argsIn...)
	if err != nil {
		log.Error(stderr)
		p.t.Fatal(fmt.Errorf("kluctl failed: %w", err))
	}
	return stdout, stderr
}

func (p *testProject) buildLocalGitUrl(repo string) string {
	relPth, err := filepath.Rel(p.baseDir, repo)
	if err != nil {
		log.Panic(err)
	}
	return fmt.Sprintf("http://localhost:%d/%s/.git", p.gitServerPort, relPth)
}
