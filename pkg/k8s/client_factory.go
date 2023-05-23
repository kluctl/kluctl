package k8s

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"net/http"
	"net/url"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"time"
)

type ClientFactory interface {
	RESTConfig() *rest.Config
	GetCA() []byte

	CloseIdleConnections()

	Mapper() meta.ResettableRESTMapper
	Client(wh rest.WarningHandler) (client.Client, error)

	DiscoveryClient() (discovery.DiscoveryInterface, error)
	CoreV1Client(wh rest.WarningHandler) (corev1.CoreV1Interface, error)
	DynamicClient(wh rest.WarningHandler) (dynamic.Interface, error)
	MetadataClient(wh rest.WarningHandler) (metadata.Interface, error)
}

type realClientFactory struct {
	ctx        context.Context
	config     *rest.Config
	httpClient *http.Client

	discoveryClient discovery.DiscoveryInterface
	mapper          meta.ResettableRESTMapper
}

func (r *realClientFactory) RESTConfig() *rest.Config {
	return r.config
}

func (r *realClientFactory) GetCA() []byte {
	return r.config.CAData
}

func (r *realClientFactory) Mapper() meta.ResettableRESTMapper {
	return r.mapper
}

func (r *realClientFactory) Client(wh rest.WarningHandler) (client.Client, error) {
	config := rest.CopyConfig(r.config)
	config.WarningHandler = wh

	return client.New(config, client.Options{
		Mapper: r.mapper,
		WarningHandler: client.WarningHandlerOptions{
			SuppressWarnings: true,
		},
	})
}

func (r *realClientFactory) DiscoveryClient() (discovery.DiscoveryInterface, error) {
	return r.discoveryClient, nil
}

func (r *realClientFactory) CoreV1Client(wh rest.WarningHandler) (corev1.CoreV1Interface, error) {
	config := rest.CopyConfig(r.config)
	config.WarningHandler = wh
	return corev1.NewForConfigAndClient(config, r.httpClient)
}

func (r *realClientFactory) DynamicClient(wh rest.WarningHandler) (dynamic.Interface, error) {
	config := rest.CopyConfig(r.config)
	config.WarningHandler = wh
	return dynamic.NewForConfigAndClient(config, r.httpClient)
}

func (r *realClientFactory) MetadataClient(wh rest.WarningHandler) (metadata.Interface, error) {
	config := rest.CopyConfig(r.config)
	config.WarningHandler = wh
	return metadata.NewForConfigAndClient(config, r.httpClient)
}

func (r *realClientFactory) CloseIdleConnections() {
	r.httpClient.CloseIdleConnections()
}

func initDiscoveryClient(ctx context.Context, config *rest.Config) (discovery.CachedDiscoveryInterface, error) {
	apiHost, err := url.Parse(config.Host)
	if err != nil {
		return nil, err
	}
	discoveryCacheDir := filepath.Join(utils.GetTmpBaseDir(ctx), "kube-cache/discovery", apiHost.Hostname())
	discovery2, err := disk.NewCachedDiscoveryClientForConfig(dynamic.ConfigFor(config), discoveryCacheDir, "", time.Hour*24)
	if err != nil {
		return nil, err
	}
	return discovery2, nil
}

func NewClientFactory(ctx context.Context, configIn *rest.Config) (ClientFactory, error) {
	restConfig := rest.CopyConfig(configIn)
	restConfig.QPS = 10
	restConfig.Burst = 20

	httpClient, err := rest.HTTPClientFor(restConfig)
	if err != nil {
		return nil, err
	}

	dc, err := initDiscoveryClient(ctx, restConfig)
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(dc)

	return &realClientFactory{
		ctx:             ctx,
		config:          restConfig,
		httpClient:      httpClient,
		discoveryClient: dc,
		mapper:          mapper,
	}, nil
}

func NewClientFactoryFromDefaultConfig(ctx context.Context, context *string) (ClientFactory, error) {
	configOverrides := &clientcmd.ConfigOverrides{}
	if context != nil {
		configOverrides.CurrentContext = *context
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		configOverrides).ClientConfig()
	if err != nil {
		return nil, err
	}

	return NewClientFactory(ctx, config)
}
