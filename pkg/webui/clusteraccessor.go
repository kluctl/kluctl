package webui

import (
	"context"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	k8s2 "github.com/kluctl/kluctl/v2/pkg/k8s"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
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
	scheme    *runtime.Scheme
	discovery discovery.DiscoveryInterface
	mapper    meta.RESTMapper
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
	ca.mutex.Lock()
	defer ca.mutex.Unlock()

	scheme := runtime.NewScheme()
	err := clientgoscheme.AddToScheme(scheme)
	if err != nil {
		return err
	}
	err = kluctlv1.AddToScheme(scheme)
	if err != nil {
		return err
	}
	ca.scheme = scheme

	httpClient, err := rest.HTTPClientFor(ca.config)
	if err != nil {
		return err
	}

	dc, err := k8s2.CreateDiscoveryClient(context.Background(), ca.config)
	if err != nil {
		return err
	}
	ca.discovery = dc

	mapper, err := apiutil.NewDynamicRESTMapper(ca.config, httpClient)
	if err != nil {
		return err
	}
	ca.mapper = mapper

	c, err := ca.getClientLocked("", nil)
	if err != nil {
		return err
	}

	var ns corev1.Namespace
	err = c.Get(context.Background(), client.ObjectKey{Name: "kube-system"}, &ns)
	if err != nil {
		return err
	}

	ca.clusterId = string(ns.UID)

	return nil
}

func (ca *clusterAccessor) getClusterId() string {
	ca.mutex.Lock()
	defer ca.mutex.Unlock()
	return ca.clusterId
}

func (ca *clusterAccessor) getClient(asUser string, asGroups []string) (client.Client, error) {
	ca.mutex.Lock()
	defer ca.mutex.Unlock()
	return ca.getClientLocked(asUser, asGroups)
}

func (ca *clusterAccessor) getClientLocked(asUser string, asGroups []string) (client.Client, error) {
	config := rest.CopyConfig(ca.config)
	config.Impersonate.UserName = asUser
	config.Impersonate.Groups = asGroups

	c, err := client.NewWithWatch(config, client.Options{
		Scheme: ca.scheme,
		Mapper: ca.mapper,
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (ca *clusterAccessor) getK(ctx context.Context, asUser string, asGroups []string) (*k8s2.K8sCluster, error) {
	ca.mutex.Lock()
	defer ca.mutex.Unlock()

	config := rest.CopyConfig(ca.config)
	config.Impersonate.UserName = asUser
	config.Impersonate.Groups = asGroups

	httpClient, err := rest.HTTPClientFor(config)
	if err != nil {
		return nil, err
	}

	cf, err := k8s2.NewClientFactory(ctx, config, httpClient, ca.discovery, ca.mapper)
	if err != nil {
		return nil, err
	}

	return k8s2.NewK8sCluster(ctx, cf, false)
}
