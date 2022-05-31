package e2e

import (
	"fmt"
	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"os"
	"path/filepath"
	"sync"
)

func createKindCluster(name string, kubeconfig string) *test_utils.KindCluster {
	k, err := test_utils.CreateKindCluster(name, kubeconfig)
	if err != nil {
		panic(err)
	}
	return k
}

func createDefaultKindCluster(num int) *test_utils.KindCluster {
	kindClusterName := os.Getenv(fmt.Sprintf("KIND_CLUSTER_NAME%d", num))
	kindKubeconfig := os.Getenv(fmt.Sprintf("KIND_KUBECONFIG%d", num))
	if kindClusterName == "" {
		kindClusterName = fmt.Sprintf("kluctl-e2e-%d", num)
	}
	if kindKubeconfig == "" {
		kindKubeconfig = filepath.Join(utils.GetTmpBaseDir(), fmt.Sprintf("kluctl-e2e-kubeconfig-%d.yml", num))
	}
	return createKindCluster(kindClusterName, kindKubeconfig)
}

func createDefaultKindClusters() (*test_utils.KindCluster, *test_utils.KindCluster) {
	var k1, k2 *test_utils.KindCluster

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		k1 = createDefaultKindCluster(1)
	}()
	go func() {
		defer wg.Done()
		k2 = createDefaultKindCluster(2)
	}()
	wg.Wait()
	return k1, k2
}

var (
	defaultKindCluster1, defaultKindCluster2 = createDefaultKindClusters()
)

func init() {
	deleteTestNamespaces(defaultKindCluster1)
	deleteTestNamespaces(defaultKindCluster2)
}
