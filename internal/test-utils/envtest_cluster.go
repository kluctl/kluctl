package test_utils

import (
	"bytes"
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/flowcontrol"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"testing"
)

type EnvTestCluster struct {
	env     envtest.Environment
	started bool

	user       *envtest.AuthenticatedUser
	Kubeconfig []byte
	Context    string
	config     *rest.Config

	HttpClient    *http.Client
	DynamicClient dynamic.Interface

	callbackServer     webhook.Server
	callbackServerStop context.CancelFunc
}

func CreateEnvTestCluster(context string) *EnvTestCluster {
	k := &EnvTestCluster{
		Context: context,
	}
	return k
}

func (k *EnvTestCluster) Start() error {
	_, err := k.env.Start()
	if err != nil {
		return err
	}
	k.started = true

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

	client, err := rest.HTTPClientFor(k.config)
	if err != nil {
		return err
	}
	k.HttpClient = client

	dynamicClient, err := dynamic.NewForConfigAndClient(k.config, k.HttpClient)
	if err != nil {
		return err
	}
	k.DynamicClient = dynamicClient

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
