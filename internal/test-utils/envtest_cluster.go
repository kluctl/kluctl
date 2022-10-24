package test_utils

import (
	"bytes"
	"context"
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"net/http"
	"os"
	"os/exec"
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

func (c *EnvTestCluster) Kubectl(args ...string) (string, string, error) {
	tmp, err := os.CreateTemp("", "")
	if err != nil {
		return "", "", err
	}
	defer func() {
		tmp.Close()
		os.Remove(tmp.Name())
	}()

	_, err = tmp.Write(c.Kubeconfig)
	if err != nil {
		return "", "", err
	}

	stdoutBuffer := &bytes.Buffer{}
	stderrBuffer := &bytes.Buffer{}
	allArgs := append([]string{fmt.Sprintf("--kubeconfig=%s", tmp.Name())}, args...)

	cmd := exec.Command(c.env.ControlPlane.KubectlPath, allArgs...)
	cmd.Stdout = stdoutBuffer
	cmd.Stderr = stderrBuffer

	err = cmd.Run()
	stdout, _ := io.ReadAll(stdoutBuffer)
	stderr, _ := io.ReadAll(stderrBuffer)
	return string(stdout), string(stderr), err
}

func (c *EnvTestCluster) KubectlMust(t *testing.T, args ...string) string {
	stdout, stderr, err := c.Kubectl(args...)
	if err != nil {
		if t != nil {
			t.Fatalf("%v, stderr=%s\n", err, stderr)
		} else {
			panic(fmt.Sprintf("%v, stderr=%s\n", err, stderr))
		}
	}
	return stdout
}

func (c *EnvTestCluster) KubectlYaml(args ...string) (*uo.UnstructuredObject, string, error) {
	args = append(args, "-oyaml")
	stdout, stderr, err := c.Kubectl(args...)
	if err != nil {
		return nil, stderr, err
	}
	ret := uo.New()
	err = yaml.ReadYamlString(stdout, ret)
	return ret, stderr, err
}

func (c *EnvTestCluster) KubectlYamlMust(t *testing.T, args ...string) *uo.UnstructuredObject {
	o, stderr, err := c.KubectlYaml(args...)
	if err != nil {
		t.Fatalf("%v, stderr=%s\n", err, stderr)
	}
	return o
}
