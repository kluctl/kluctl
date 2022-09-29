package e2e

import (
	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"sync"
)

var defaultCluster1 = test_utils.CreateEnvTestCluster("cluster1")
var defaultCluster2 = test_utils.CreateEnvTestCluster("cluster2")
var defaultKindCluster1VaultPort, defaultKindCluster2VaultPort int

func init() {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := defaultCluster1.Start()
		if err != nil {
			panic(err)
		}
	}()
	go func() {
		defer wg.Done()
		err := defaultCluster2.Start()
		if err != nil {
			panic(err)
		}
	}()
	wg.Wait()
}
