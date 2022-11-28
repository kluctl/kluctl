package e2e

import (
	"github.com/kluctl/kluctl/v2/e2e/test-utils"
	"github.com/kluctl/kluctl/v2/e2e/test_resources"
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
}
