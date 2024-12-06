package k8s

import (
	"context"
	"fmt"
	"k8s.io/client-go/rest"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type k8sClients struct {
	k          *K8sCluster
	clientPool chan *parallelClientEntry
	count      int
}

type parallelClientEntry struct {
	config     *rest.Config
	httpClient *http.Client
	client     client.Client

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

func newK8sClients(k *K8sCluster, count int) (*k8sClients, error) {
	kc := &k8sClients{
		k:          k,
		clientPool: make(chan *parallelClientEntry, count),
		count:      count,
	}

	for i := 0; i < count; i++ {
		p, err := kc.newClientEntry()
		if err != nil {
			return nil, err
		}

		kc.clientPool <- p
	}
	return kc, nil
}

func (kc *k8sClients) newClientEntry() (*parallelClientEntry, error) {
	p := &parallelClientEntry{}

	p.config = rest.CopyConfig(kc.k.config)
	p.config.QPS = 10
	p.config.Burst = 20
	p.config.WarningHandler = p

	var err error
	p.httpClient, err = rest.HTTPClientFor(p.config)

	p.client, err = client.New(p.config, client.Options{
		HTTPClient: p.httpClient,
		Mapper:     kc.k.mapper,
	})
	if err != nil {
		return nil, err
	}

	return p, nil
}

func (k *k8sClients) close() {
	if k.clientPool != nil {
		for i := 0; i < k.count; i++ {
			p := <-k.clientPool
			p.httpClient.CloseIdleConnections()
		}
	}
	k.clientPool = nil
	k.count = 0
}

func (k *k8sClients) withClientFromPool(ctx context.Context, cb func(p *parallelClientEntry) error) ([]ApiWarning, error) {
	select {
	case p := <-k.clientPool:
		defer func() { k.clientPool <- p }()
		p.warnings = nil
		err := cb(p)
		return append([]ApiWarning(nil), p.warnings...), err
	case <-ctx.Done():
		return nil, fmt.Errorf("failed waiting for free client: %w", ctx.Err())
	}
}

func (k *k8sClients) withCClientFromPool(ctx context.Context, dryRun bool, cb func(c client.Client) error) ([]ApiWarning, error) {
	return k.withClientFromPool(ctx, func(p *parallelClientEntry) error {
		c := p.client
		if dryRun {
			c = client.NewDryRunClient(c)
		}
		return cb(c)
	})
}
