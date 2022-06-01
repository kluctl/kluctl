package test_utils

import (
	"fmt"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"net/url"
	"os"
	"os/exec"
	"sigs.k8s.io/kind/pkg/apis/config/v1alpha4"
	"sigs.k8s.io/kind/pkg/cluster"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
	"testing"
	"time"
)

type KindCluster struct {
	Name       string
	Context    string
	Kubeconfig string
	config     *rest.Config
}

func CreateKindCluster(name, apiServerHost string, apiServerPort int, extraPorts map[int]int, kubeconfigPath string) (*KindCluster, error) {
	provider := cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))

	c := &KindCluster{
		Name:       name,
		Context:    fmt.Sprintf("kind-%s", name),
		Kubeconfig: kubeconfigPath,
	}

	n, err := provider.ListNodes(name)
	if err != nil {
		return nil, err
	}
	if len(n) == 0 {
		if err := kindCreate(name, apiServerHost, apiServerPort, extraPorts, kubeconfigPath); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// Delete removes the cluster from kind. The cluster may not be deleted straight away - this only issues a delete command
func (c *KindCluster) Delete() error {
	provider := cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
	return provider.Delete(c.Name, c.Kubeconfig)
}

// RESTConfig returns K8s client config to pass to clientset objects
func (c *KindCluster) RESTConfig() *rest.Config {
	if c.config == nil {
		var err error
		c.config, err = clientcmd.BuildConfigFromFlags("", c.Kubeconfig)
		if err != nil {
			panic(err)
		}
	}
	return c.config
}

func (c *KindCluster) Kubectl(args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", c.Kubeconfig))

	stdout, err := cmd.Output()
	return string(stdout), err
}

func (c *KindCluster) KubectlMust(t *testing.T, args ...string) string {
	stdout, err := c.Kubectl(args...)
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			t.Fatalf("%v, stderr=%s\n", err, string(e.Stderr))
		} else {
			t.Fatal(err)
		}
	}
	return stdout
}

func (c *KindCluster) KubectlYaml(args ...string) (*uo.UnstructuredObject, error) {
	args = append(args, "-oyaml")
	stdout, err := c.Kubectl(args...)
	if err != nil {
		return nil, err
	}
	ret := uo.New()
	err = yaml.ReadYamlString(stdout, ret)
	return ret, err
}

func (c *KindCluster) KubectlYamlMust(t *testing.T, args ...string) *uo.UnstructuredObject {
	o, err := c.KubectlYaml(args...)
	if err != nil {
		if e, ok := err.(*exec.ExitError); ok {
			t.Fatalf("%v, stderr=%s\n", err, string(e.Stderr))
		} else {
			t.Fatal(err)
		}
	}
	return o
}

// kindCreate creates the kind cluster. It will retry up to 10 times if cluster creation fails.
func kindCreate(name string, apiServerHost string, apiServerPort int, extraPorts map[int]int, kubeconfig string) error {
	var err error
	for i := 0; i < 10; i++ {
		err = kindCreate2(name, apiServerHost, apiServerPort, extraPorts, kubeconfig)
		if err == nil {
			return nil
		}
	}
	return err
}

func kindCreate2(name string, apiServerHost string, apiServerPort int, extraPorts map[int]int, kubeconfig string) error {
	fmt.Printf("🌧️  Creating kind cluster %s with apiServerHost=%s, apiServerPort=%d, extraPorts=%v...\n", name, apiServerHost, apiServerPort, extraPorts)
	provider := cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
	config := v1alpha4.Cluster{
		Name: name,
		Nodes: []v1alpha4.Node{{
			Role: "control-plane",
		}},
		Networking: v1alpha4.Networking{
			APIServerAddress: "0.0.0.0",
			APIServerPort:    int32(apiServerPort),
		},
	}
	for hostPort, containerPort := range extraPorts {
		config.Nodes[0].ExtraPortMappings = append(config.Nodes[0].ExtraPortMappings, v1alpha4.PortMapping{
			ContainerPort: int32(containerPort),
			HostPort:      int32(hostPort),
			ListenAddress: "0.0.0.0",
			Protocol:      "TCP",
		})
	}

	err := provider.Create(
		name,
		cluster.CreateWithV1Alpha4Config(&config),
		cluster.CreateWithNodeImage(""),
		cluster.CreateWithRetain(false),
		cluster.CreateWithWaitForReady(time.Duration(0)),
		cluster.CreateWithKubeconfigPath(kubeconfig),
		cluster.CreateWithDisplayUsage(false),
	)
	if err != nil {
		return err
	}

	kcfg, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return err
	}

	c := kcfg.Clusters[fmt.Sprintf("kind-%s", name)]

	u, err := url.Parse(c.Server)
	if err != nil {
		return err
	}

	// override api server host and disable TLS verification
	// this is needed to make it work with remote docker hosts
	c.InsecureSkipTLSVerify = true
	c.CertificateAuthorityData = nil
	c.Server = fmt.Sprintf("https://%s:%s", apiServerHost, u.Port())

	err = clientcmd.WriteToFile(*kcfg, kubeconfig)
	if err != nil {
		return err
	}

	return nil
}
