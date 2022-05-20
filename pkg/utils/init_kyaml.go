package utils

import (
	"sigs.k8s.io/kustomize/kyaml/openapi"
	yaml2 "sigs.k8s.io/kustomize/kyaml/yaml"
	"sync"
)

var openapiInitDoneMutex sync.Mutex
var openapiInitDoneOnce sync.Once

func WaitForOpenapiInitDone() {
	openapiInitDoneOnce.Do(func() {
		openapiInitDoneMutex.Lock()
		openapiInitDoneMutex.Unlock()
	})
}

func init() {
	openapiInitDoneMutex.Lock()
	go func() {
		// we do a single call to IsNamespaceScoped to enforce openapi schema initialization
		// this is required here to ensure that it is later not done in parallel which would cause race conditions
		openapi.IsNamespaceScoped(yaml2.TypeMeta{
			APIVersion: "",
			Kind:       "ConfigMap",
		})
		openapiInitDoneMutex.Unlock()
	}()
}
