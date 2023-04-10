package webui

import (
	"context"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sync"
	"time"
)

type clusterAccessorManager struct {
	ctx       context.Context
	accessors []*clusterAccessor
}

type clusterAccessor struct {
	ctx       context.Context
	config    *rest.Config
	client    client.Client
	k         *k8s2.K8sCluster
	clusterId string
	mutex     sync.Mutex
}

func (cam *clusterAccessorManager) add(config *rest.Config) {
	cam.accessors = append(cam.accessors, &clusterAccessor{
		ctx:    cam.ctx,
		config: config,
	})
}

func (cam *clusterAccessorManager) start() {
	for _, ca := range cam.accessors {
		ca.start()
	}
}

func (cam *clusterAccessorManager) getForClusterId(clusterId string) *clusterAccessor {
	for _, ca := range cam.accessors {
		if ca.getClusterId() == clusterId {
			return ca
		}
	}
	return nil
}

func (ca *clusterAccessor) start() {
	go ca.initClient()
}

func (ca *clusterAccessor) initClient() {
	for {
		err := ca.tryInitClient()
		if err == nil {
			break
		}
		time.Sleep(5 * time.Second)
	}
}

func (ca *clusterAccessor) tryInitClient() error {
	var err error
	c, err := client.New(ca.config, client.Options{})
	if err != nil {
		return err
	}
	var ns corev1.Namespace
	err = c.Get(context.Background(), client.ObjectKey{Name: "kube-system"}, &ns)
	if err != nil {
		return err
	}

	cf, err := k8s2.NewClientFactory(context.Background(), ca.config)
	if err != nil {
		return err
	}

	k, err := k8s2.NewK8sCluster(context.Background(), cf, true)
	if err != nil {
		return err
	}

	ca.mutex.Lock()
	defer ca.mutex.Unlock()
	ca.client = c
	ca.k = k
	ca.clusterId = string(ns.UID)

	return nil
}

func (ca *clusterAccessor) getClusterId() string {
	ca.mutex.Lock()
	defer ca.mutex.Unlock()
	return ca.clusterId
}

func (ca *clusterAccessor) getClient() client.Client {
	ca.mutex.Lock()
	defer ca.mutex.Unlock()
	return ca.client
}

func (ca *clusterAccessor) getK() *k8s2.K8sCluster {
	ca.mutex.Lock()
	defer ca.mutex.Unlock()
	return ca.k
}
