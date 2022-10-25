package e2e

import (
	test_utils "github.com/kluctl/kluctl/v2/internal/test-utils"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sync"
)

var defaultCluster1 = test_utils.CreateEnvTestCluster("cluster1")
var defaultCluster2 = test_utils.CreateEnvTestCluster("cluster2")

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
	wg.Wait()
}
