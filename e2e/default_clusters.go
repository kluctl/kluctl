package e2e

import "C"
import (
	"fmt"
	"github.com/imdario/mergo"
	"github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_resources"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"runtime"
	"sync"
	"testing"
)

var defaultCluster1 = test_utils.CreateEnvTestCluster("cluster1")
var defaultCluster2 = test_utils.CreateEnvTestCluster("cluster2")
var mergedKubeconfig string

func init() {
	if isCallKluctlHack() {
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := defaultCluster1.Start()
		if err != nil {
			panic(err)
		}
		test_resources.ApplyYaml("sealed-secrets.yaml", defaultCluster1)
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
		test_resources.ApplyYaml("sealed-secrets.yaml", defaultCluster2)
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

func setMergedKubeconfigContext(t *testing.T, newContext string) {
	tmpKubeconfig, err := os.CreateTemp(t.TempDir(), "kubeconfig-")
	if err != nil {
		t.Fatal(err)
	}
	_ = tmpKubeconfig.Close()

	kcfg, err := clientcmd.LoadFromFile(mergedKubeconfig)
	if err != nil {
		t.Fatal(err)
	}
	kcfg.CurrentContext = newContext
	err = clientcmd.WriteToFile(*kcfg, tmpKubeconfig.Name())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", tmpKubeconfig.Name())
}
