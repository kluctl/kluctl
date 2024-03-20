package test_utils

import (
	"bytes"
	"context"
	"fmt"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sync"
	"testing"
)

type EnvTestCluster struct {
	CRDDirectoryPaths []string

	env     envtest.Environment
	started bool

	user       *envtest.AuthenticatedUser
	Kubeconfig []byte
	Context    string
	config     *rest.Config

	HttpClient    *http.Client
	DynamicClient dynamic.Interface
	Client        client.WithWatch
	ServerVersion *version.Info

	callbackServer     webhook.Server
	callbackServerStop context.CancelFunc

	webhookHandlers      []*CallbackHandlerEntry
	webhookHandlersMutex sync.Mutex
}

func CreateEnvTestCluster(context string) *EnvTestCluster {
	k := &EnvTestCluster{
		Context: context,
		env: envtest.Environment{
			Scheme: runtime.NewScheme(),
		},
	}
	return k
}

func (k *EnvTestCluster) Start() error {
	k.env.CRDDirectoryPaths = k.CRDDirectoryPaths

	_, err := k.env.Start()
	if err != nil {
		return err
	}
	k.started = true

	_ = kluctlv1.AddToScheme(k.env.Scheme)
	_ = corev1.AddToScheme(k.env.Scheme)

	err = k.startCallbackServer()
	if err != nil {
		return err
	}

	user, err := k.env.AddUser(envtest.User{Name: "default", Groups: []string{"system:masters"}}, &rest.Config{})
	if err != nil {
		return err
	}
	k.user = user

	k.config = user.Config()
	k.config.RateLimiter = flowcontrol.NewFakeAlwaysRateLimiter()

	kcfg, err := user.KubeConfig()
	if err != nil {
		return err
	}

	kcfg = bytes.ReplaceAll(kcfg, []byte("envtest"), []byte(k.Context))

	k.Kubeconfig = kcfg

	httpClient, err := rest.HTTPClientFor(k.config)
	if err != nil {
		return err
	}
	k.HttpClient = httpClient

	dynamicClient, err := dynamic.NewForConfigAndClient(k.config, k.HttpClient)
	if err != nil {
		return err
	}
	k.DynamicClient = dynamicClient

	c, err := client.NewWithWatch(k.config, client.Options{
		HTTPClient: httpClient,
		Scheme:     k.env.Scheme,
	})
	k.Client = c

	discoveryClient, err := discovery.NewDiscoveryClientForConfigAndClient(k.config, httpClient)
	if err != nil {
		return err
	}
	k.ServerVersion, err = discoveryClient.ServerVersion()
	if err != nil {
		return err
	}

	return nil
}

func (c *EnvTestCluster) Stop() {
	if c.started {
		_ = c.env.Stop()
		c.started = false
	}
	if c.callbackServerStop != nil {
		c.callbackServerStop()
		c.callbackServerStop = nil
	}
}

func (c *EnvTestCluster) AddUser(user envtest.User, baseConfig *rest.Config) (*envtest.AuthenticatedUser, error) {
	return c.env.AddUser(user, baseConfig)
}

// RESTConfig returns K8s client config to pass to clientset objects
func (c *EnvTestCluster) RESTConfig() *rest.Config {
	return c.config
}

func (c *EnvTestCluster) Get(gvr schema.GroupVersionResource, namespace string, name string) (*uo.UnstructuredObject, error) {
	x, err := c.DynamicClient.Resource(gvr).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return uo.FromUnstructured(x), nil
}

func (c *EnvTestCluster) MustGet(t *testing.T, gvr schema.GroupVersionResource, namespace string, name string) *uo.UnstructuredObject {
	x, err := c.Get(gvr, namespace, name)
	if err != nil {
		t.Fatalf("error while getting %s/%s/%s: %s", gvr.String(), namespace, name, err.Error())
	}
	return x
}

func (c *EnvTestCluster) MustGetCoreV1(t *testing.T, resource string, namespace string, name string) *uo.UnstructuredObject {
	return c.MustGet(t, schema.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: resource,
	}, namespace, name)
}

func (c *EnvTestCluster) List(gvr schema.GroupVersionResource, namespace string, labels map[string]string) ([]*uo.UnstructuredObject, error) {
	labelSelector := ""
	if len(labels) != 0 {
		for k, v := range labels {
			if labelSelector != "" {
				labelSelector += ","
			}
			labelSelector += fmt.Sprintf("%s=%s", k, v)
		}
	}

	l, err := c.DynamicClient.Resource(gvr).
		Namespace(namespace).
		List(context.Background(), metav1.ListOptions{
			LabelSelector: labelSelector,
		})
	if err != nil {
		return nil, err
	}
	var ret []*uo.UnstructuredObject
	for _, x := range l.Items {
		x := x
		ret = append(ret, uo.FromUnstructured(&x))
	}
	return ret, nil
}

func (c *EnvTestCluster) Apply(o *uo.UnstructuredObject) error {
	var x unstructured.Unstructured
	err := o.ToStruct(&x)
	if err != nil {
		return err
	}
	return c.Client.Patch(context.Background(), &x, client.Apply, client.FieldOwner("envtestcluster"))
}

func (c *EnvTestCluster) MustApply(t *testing.T, o *uo.UnstructuredObject) {
	err := c.Apply(o)
	if err != nil {
		t.Fatal(err)
	}
}
