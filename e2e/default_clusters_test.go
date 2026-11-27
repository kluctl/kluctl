package e2e

import (
	"dario.cat/mergo"
	"errors"
	"fmt"
	"github.com/kluctl/kluctl/v2/e2e/test-utils"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

var defaultCluster1 = test_utils.CreateEnvTestCluster("cluster1")
var defaultCluster2 = test_utils.CreateEnvTestCluster("cluster2")
var gitopsCluster = test_utils.CreateEnvTestCluster("gitops")
var mergedKubeconfig string

var defaultClusters = []*test_utils.EnvTestCluster{defaultCluster1, defaultCluster2, gitopsCluster}

func startDefaultClusters() {
	defaultCluster2.InitWebhookCallback(schema.GroupVersionResource{
		Version: "v1", Resource: "configmaps",
	}, true)
	gitopsCluster.CRDDirectoryPaths = []string{"../config/crd/bases"}

	var wg sync.WaitGroup
	startErrors := make([]error, len(defaultClusters))
	for i, cluster := range defaultClusters {
		wg.Go(func() {
			if err := cluster.Start(); err != nil {
				startErrors[i] = fmt.Errorf("start envtest cluster %s: %w", cluster.Context, err)
			}
		})
	}
	// Let all starts finish before TestMain can tear down the clusters.
	wg.Wait()
	if err := errors.Join(startErrors...); err != nil {
		panic(err)
	}

	tmpKubeconfig, err := os.CreateTemp("", "")
	if err != nil {
		panic(err)
	}
	_ = tmpKubeconfig.Close()
	mergedKubeconfig = tmpKubeconfig.Name()
	for _, cluster := range defaultClusters {
		mergeKubeconfig(mergedKubeconfig, cluster.Kubeconfig)
	}
	_ = os.Setenv("KUBECONFIG", mergedKubeconfig)

	_, _ = fmt.Fprintf(os.Stderr, "KUBECONFIG=%s\n", mergedKubeconfig)
}

func stopDefaultClusters() {
	for _, cluster := range defaultClusters {
		if err := cluster.Stop(); err != nil {
			fmt.Fprintf(os.Stderr, "stop envtest cluster %s: %v\n", cluster.Context, err)
		}
	}
	if mergedKubeconfig != "" {
		_ = os.Remove(mergedKubeconfig)
	}
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
