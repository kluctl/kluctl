package e2e

import (
	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"sync"
)

var defaultKindCluster1, defaultKindCluster2 *test_utils.EnvTestCluster
var defaultKindCluster1VaultPort, defaultKindCluster2VaultPort int

func init() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		var err error
		defaultKindCluster1, err = test_utils.CreateEnvTestCluster("cluster1")
		if err != nil {
			panic(err)
		}
	}()
	go func() {
		defer wg.Done()
		var err error
		defaultKindCluster2, err = test_utils.CreateEnvTestCluster("cluster2")
		if err != nil {
			panic(err)
		}
	}()
	wg.Wait()
}
