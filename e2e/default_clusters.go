package e2e

import (
	"fmt"
	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

func createDefaultKindCluster(num int) (*test_utils.KindCluster, int) {
	kindClusterName := os.Getenv(fmt.Sprintf("KIND_CLUSTER_NAME%d", num))
	kindApiHost := os.Getenv(fmt.Sprintf("KIND_API_HOST%d", num))
	kindApiPort := os.Getenv(fmt.Sprintf("KIND_API_PORT%d", num))
	kindExtraPortsOffset := os.Getenv(fmt.Sprintf("KIND_EXTRA_PORTS_OFFSET%d", num))
	kindKubeconfig := os.Getenv(fmt.Sprintf("KIND_KUBECONFIG%d", num))
	if kindClusterName == "" {
		kindClusterName = fmt.Sprintf("kluctl-e2e-%d", num)
	}
	if kindApiHost == "" {
		kindApiHost = "localhost"
	}
	if kindExtraPortsOffset == "" {
		kindExtraPortsOffset = fmt.Sprintf("%d", 30000+num*1000)
	}
	if kindKubeconfig == "" {
		kindKubeconfig = filepath.Join(utils.GetTmpBaseDir(), fmt.Sprintf("kluctl-e2e-kubeconfig-%d.yml", num))
	}

	var err error
	var kindApiPortInt, kindExtraPortsOffsetInt int64

	if kindApiPort != "" {
		kindApiPortInt, err = strconv.ParseInt(kindApiPort, 0, 32)
		if err != nil {
			panic(err)
		}
	}
	kindExtraPortsOffsetInt, err = strconv.ParseInt(kindExtraPortsOffset, 0, 32)
	if err != nil {
		panic(err)
	}

	vaultPort := int(kindExtraPortsOffsetInt) + 0

	kindExtraPorts := map[int]int{
		vaultPort: 30000,
	}

	k, err := test_utils.CreateKindCluster(kindClusterName, kindApiHost, int(kindApiPortInt), kindExtraPorts, kindKubeconfig)
	if err != nil {
		panic(err)
	}
	return k, vaultPort
}

var defaultKindCluster1, defaultKindCluster2 *test_utils.KindCluster
var defaultKindCluster1VaultPort, defaultKindCluster2VaultPort int

func init() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		defaultKindCluster1, defaultKindCluster1VaultPort = createDefaultKindCluster(1)
		deleteTestNamespaces(defaultKindCluster1)
	}()
	go func() {
		defer wg.Done()
		defaultKindCluster2, defaultKindCluster2VaultPort = createDefaultKindCluster(2)
		deleteTestNamespaces(defaultKindCluster2)
	}()
	wg.Wait()
}
