package k8s

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/disk"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

func CreateDiscoveryAndMapper(ctx context.Context, config *rest.Config) (discovery.CachedDiscoveryInterface, meta.RESTMapper, error) {
	apiHost, err := url.Parse(config.Host)
	if err != nil {
		return nil, nil, err
	}
	discoveryCacheDir := filepath.Join(utils.GetTmpBaseDir(ctx), "kube-cache/discovery", strings.ReplaceAll(apiHost.Host, ":", "-"))
	discovery2, err := disk.NewCachedDiscoveryClientForConfig(dynamic.ConfigFor(config), discoveryCacheDir, "", time.Hour*24)
	if err != nil {
		return nil, nil, err
	}

	mapper := restmapper.NewDeferredDiscoveryRESTMapper(discovery2)

	return discovery2, mapper, nil
}
