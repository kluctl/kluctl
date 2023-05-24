package k8s

import (
	"context"
	"fmt"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type k8sClients struct {
	ctx           context.Context
	clientFactory ClientFactory
	clientPool    chan *parallelClientEntry
	count         int
}

type parallelClientEntry struct {
	client client.Client
	corev1 corev1.CoreV1Interface

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

func newK8sClients(ctx context.Context, clientFactory ClientFactory, count int) (*k8sClients, error) {
	var err error

	k := &k8sClients{
		ctx:           ctx,
		clientFactory: clientFactory,
		clientPool:    make(chan *parallelClientEntry, count),
		count:         count,
	}

	for i := 0; i < count; i++ {
		p := &parallelClientEntry{}

		p.client, err = clientFactory.Client(p)
		if err != nil {
			return nil, err
		}

		p.corev1, err = clientFactory.CoreV1Client(p)
		if err != nil {
			return nil, err
		}

		k.clientPool <- p
	}
	return k, nil
}

func (k *k8sClients) close() {
	k.clientFactory.CloseIdleConnections()
	if k.clientPool != nil {
		for i := 0; i < k.count; i++ {
			_ = <-k.clientPool
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

func (k *k8sClients) withCClientFromPool(dryRun bool, cb func(c client.Client) error) ([]ApiWarning, error) {
	return k.withClientFromPool(func(p *parallelClientEntry) error {
		c := p.client
		if dryRun {
			c = client.NewDryRunClient(c)
		}
		return cb(c)
	})
}
