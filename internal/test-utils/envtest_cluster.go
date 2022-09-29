package test_utils

import (
	"bytes"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
)

type EnvTestCluster struct {
	env envtest.Environment

	user       *envtest.AuthenticatedUser
	Kubeconfig []byte
	Context    string
	config     *rest.Config
}

func CreateEnvTestCluster(context string) (*EnvTestCluster, error) {
	k := &EnvTestCluster{
		Context: context,
	}

	_, err := k.env.Start()
	if err != nil {
		return nil, err
	}

	isOk := false
	defer func() {
		if !isOk {
			_ = k.env.Stop()
		}
	}()

	user, err := k.env.AddUser(envtest.User{Name: "default", Groups: []string{"system:masters"}}, &rest.Config{})
	if err != nil {
		return nil, err
	}
	k.user = user

	kcfg, err := user.KubeConfig()
	if err != nil {
		return nil, err
	}
	
	kcfg = bytes.ReplaceAll(kcfg, []byte("envtest"), []byte(context))

	k.Kubeconfig = kcfg

	isOk = true
	return k, nil
}

// RESTConfig returns K8s client config to pass to clientset objects
func (c *EnvTestCluster) RESTConfig() *rest.Config {
	return c.config
}

func (c *EnvTestCluster) Kubectl(args ...string) (string, string, error) {
	kctl, err := c.user.Kubectl()
	if err != nil {
		return "", "", err
	}

	stdout1, stderr1, err := kctl.Run(args...)
	stdout, _ := io.ReadAll(stdout1)
	stderr, _ := io.ReadAll(stderr1)
	return string(stdout), string(stderr), err
}

func (c *EnvTestCluster) KubectlMust(t *testing.T, args ...string) string {
	stdout, stderr, err := c.Kubectl(args...)
	if err != nil {
		t.Fatalf("%v, stderr=%s\n", err, stderr)
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
