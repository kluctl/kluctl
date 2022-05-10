package k8s

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"net/http"
)

type k8sClients struct {
	ctx        context.Context
	restConfig *rest.Config
	clientPool chan *parallelClientEntry
	count      int
}

type parallelClientEntry struct {
	http           *http.Client
	corev1         *corev1.CoreV1Client
	dynamicClient  dynamic.Interface
	metadataClient metadata.Interface

	warnings []ApiWarning
}

type ApiWarning struct {
	Code  int
	Agent string
	Text  string
}

func (p *parallelClientEntry) HandleWarningHeader(code int, agent string, text string) {
	p.warnings = append(p.warnings, ApiWarning{
		Code:  code,
		Agent: agent,
		Text:  text,
	})
}

func newK8sClients(ctx context.Context, restConfig *rest.Config, count int) (*k8sClients, error) {
	var err error

	k := &k8sClients{
		ctx:        ctx,
		restConfig: restConfig,
		clientPool: make(chan *parallelClientEntry, count),
		count:      count,
	}

	for i := 0; i < count; i++ {
		p := &parallelClientEntry{}
		config := rest.CopyConfig(k.restConfig)
		config.WarningHandler = p

		p.http, err = rest.HTTPClientFor(config)
		if err != nil {
			return nil, err
		}

		p.corev1, err = corev1.NewForConfigAndClient(config, p.http)
		if err != nil {
			return nil, err
		}

		p.dynamicClient, err = dynamic.NewForConfigAndClient(config, p.http)
		if err != nil {
			return nil, err
		}

		p.metadataClient, err = metadata.NewForConfigAndClient(config, p.http)
		if err != nil {
			return nil, err
		}

		k.clientPool <- p
	}
	return k, nil
}

func (k *k8sClients) close() {
	if k.clientPool != nil {
		for i := 0; i < k.count; i++ {
			p := <-k.clientPool
			p.http.CloseIdleConnections()
		}
	}
	k.clientPool = nil
	k.count = 0
}

func (k *k8sClients) withClientFromPool(cb func(p *parallelClientEntry) error) ([]ApiWarning, error) {
	select {
	case p := <-k.clientPool:
		defer func() { k.clientPool <- p }()
		p.warnings = nil
		err := cb(p)
		return append([]ApiWarning(nil), p.warnings...), err
	case <-k.ctx.Done():
		return nil, fmt.Errorf("failed waiting for free client: %w", k.ctx.Err())
	}
}

func (k *k8sClients) withDynamicClientForGVR(gvr *schema.GroupVersionResource, namespace string, cb func(r dynamic.ResourceInterface) error) ([]ApiWarning, error) {
	return k.withClientFromPool(func(p *parallelClientEntry) error {
		if namespace != "" {
			return cb(p.dynamicClient.Resource(*gvr).Namespace(namespace))
		} else {
			return cb(p.dynamicClient.Resource(*gvr))
		}
	})
}

func (k *k8sClients) withMetadataClientForGVR(gvr *schema.GroupVersionResource, namespace string, cb func(r metadata.ResourceInterface) error) ([]ApiWarning, error) {
	return k.withClientFromPool(func(p *parallelClientEntry) error {
		if namespace != "" {
			return cb(p.metadataClient.Resource(*gvr).Namespace(namespace))
		} else {
			return cb(p.metadataClient.Resource(*gvr))
		}
	})
}

func (k *k8sClients) withDynamicClientForGVK(resources *k8sResources, gvk schema.GroupVersionKind, namespace string, cb func(r dynamic.ResourceInterface) error) ([]ApiWarning, error) {
	gvr, err := resources.GetGVRForGVK(gvk)
	if err != nil {
		return nil, err
	}
	return k.withDynamicClientForGVR(gvr, namespace, cb)
}

func (k *k8sClients) withMetadataClientForGVK(resources *k8sResources, gvk schema.GroupVersionKind, namespace string, cb func(r metadata.ResourceInterface) error) ([]ApiWarning, error) {
	gvr, err := resources.GetGVRForGVK(gvk)
	if err != nil {
		return nil, err
	}
	return k.withMetadataClientForGVR(gvr, namespace, cb)
}
