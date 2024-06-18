package e2e

import (
	"fmt"
	"github.com/imdario/mergo"
	"github.com/kluctl/kluctl/v2/e2e/test-utils"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var defaultCluster1 = test_utils.CreateEnvTestCluster("cluster1")
var defaultCluster2 = test_utils.CreateEnvTestCluster("cluster2")
var gitopsCluster = test_utils.CreateEnvTestCluster("gitops")
var mergedKubeconfig string

func init() {
	if isCallKluctl() {
		return
	}

	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		err := defaultCluster1.Start()
		if err != nil {
			panic(err)
		}
	}()
	go func() {
		defer wg.Done()
		defaultCluster2.InitWebhookCallback(schema.GroupVersionResource{
			Version: "v1", Resource: "configmaps",
		}, true)
		err := defaultCluster2.Start()
		if err != nil {
			panic(err)
		}
	}()
	go func() {
		defer wg.Done()
		gitopsCluster.CRDDirectoryPaths = []string{"../config/crd/bases"}
		err := gitopsCluster.Start()
		if err != nil {
			panic(err)
		}
	}()
	wg.Wait()

	tmpKubeconfig, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}
	_ = tmpKubeconfig.Close()
	runtime.SetFinalizer(defaultCluster1, func(_ *test_utils.EnvTestCluster) {
		_ = os.Remove(tmpKubeconfig.Name())
	})

	mergeKubeconfig(tmpKubeconfig.Name(), defaultCluster1.Kubeconfig)
	mergeKubeconfig(tmpKubeconfig.Name(), defaultCluster2.Kubeconfig)
	mergeKubeconfig(tmpKubeconfig.Name(), gitopsCluster.Kubeconfig)

	mergedKubeconfig = tmpKubeconfig.Name()
	_ = os.Setenv("KUBECONFIG", mergedKubeconfig)

	_, _ = fmt.Fprintf(os.Stderr, "KUBECONFIG=%s\n", mergedKubeconfig)
}

func mergeKubeconfig(path string, kubeconfig []byte) {
	mkcfg, err := clientcmd.LoadFromFile(path)
	if err != nil {
		panic(err)
	}

	nkcfg, err := clientcmd.Load(kubeconfig)
	if err != nil {
		panic(err)
	}

	err = mergo.Merge(mkcfg, nkcfg)
	if err != nil {
		panic(err)
	}

	err = clientcmd.WriteToFile(*mkcfg, path)
	if err != nil {
		panic(err)
	}
}

func getKubeconfigTmpFile(t *testing.T, content []byte) string {
	fname := filepath.Join(t.TempDir(), "kubeconfig")
	err := os.WriteFile(fname, content, 0600)
	if err != nil {
		t.Fatal(err)
	}
	return fname
}

func setKubeconfigString(t *testing.T, content []byte) {
	tmpKubeconfig := getKubeconfigTmpFile(t, content)
	t.Logf("set KUBECONFIG=%s\n", tmpKubeconfig)
	t.Setenv("KUBECONFIG", tmpKubeconfig)
}

func setKubeconfig(t *testing.T, config api.Config) {
	content, err := clientcmd.Write(config)
	if err != nil {
		t.Fatal(err)
	}
	setKubeconfigString(t, content)
}

func setMergedKubeconfigContext(t *testing.T, newContext string) {
	kcfg, err := clientcmd.LoadFromFile(mergedKubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	kcfg.CurrentContext = newContext
	setKubeconfig(t, *kcfg)
}
