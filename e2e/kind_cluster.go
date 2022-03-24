package e2e

import (
	"fmt"
	"github.com/kluctl/kluctl/pkg/utils"
	"github.com/kluctl/kluctl/pkg/utils/uo"
	"github.com/kluctl/kluctl/pkg/yaml"
	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"os/exec"
	"path/filepath"
	"sigs.k8s.io/kind/pkg/cluster"
	kindcmd "sigs.k8s.io/kind/pkg/cmd"
	"testing"
	"time"
)

type KindCluster struct {
	Name       string
	kubeconfig string
	config     *rest.Config
}

func CreateKindCluster(name, kubeconfigPath string) (*KindCluster, error) {
	provider := cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))

	c := &KindCluster{
		Name:       name,
		kubeconfig: kubeconfigPath,
	}

	n, err := provider.ListNodes(name)
	if err != nil {
		return nil, err
	}
	if len(n) == 0 {
		if err := kindCreate(name, kubeconfigPath); err != nil {
			return nil, err
		}
	}

	return c, nil
}

// Delete removes the cluster from kind. The cluster may not be deleted straight away - this only issues a delete command
func (c *KindCluster) Delete() error {
	provider := cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
	return provider.Delete(c.Name, c.kubeconfig)
}

// Kubeconfig returns the path to the cluster kubeconfig
func (c *KindCluster) Kubeconfig() string {
	return c.kubeconfig
}

// RESTConfig returns K8s client config to pass to clientset objects
func (c *KindCluster) RESTConfig() *rest.Config {
	if c.config == nil {
		var err error
		c.config, err = clientcmd.BuildConfigFromFlags("", c.Kubeconfig())
		if err != nil {
			panic(err)
		}
	}
	return c.config
}

func (c *KindCluster) Kubectl(args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", c.kubeconfig))

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
func kindCreate(name, kubeconfig string) error {

	fmt.Printf("🌧️  Creating kind cluster %s...\n", name)
	provider := cluster.NewProvider(cluster.ProviderWithLogger(kindcmd.NewLogger()))
	attempts := 0
	maxAttempts := 10
	for {
		err := provider.Create(
			name,
			cluster.CreateWithNodeImage(""),
			cluster.CreateWithRetain(false),
			cluster.CreateWithWaitForReady(time.Duration(0)),
			cluster.CreateWithKubeconfigPath(kubeconfig),
			cluster.CreateWithDisplayUsage(false),
		)
		if err == nil {
			return nil
		}

		fmt.Printf("Error bringing up cluster, will retry (attempt %d): %v", attempts, err)
		attempts++
		if attempts >= maxAttempts {
			return errors.Wrapf(err, "Error bringing up cluster, exceeded max attempts (%d)", attempts)
		}
	}
}

func createKindCluster(name string, kubeconfig string) *KindCluster {
	k, err := CreateKindCluster(name, kubeconfig)
	if err != nil {
		log.Fatal(err)
	}
	return k
}

func createDefaultKindCluster() *KindCluster {
	kindClusterName := os.Getenv("KIND_CLUSTER_NAME")
	kindKubeconfig := os.Getenv("KIND_KUBECONFIG")
	if kindClusterName == "" {
		kindClusterName = "kluctl-e2e"
	}
	if kindKubeconfig == "" {
		kindKubeconfig = filepath.Join(utils.GetTmpBaseDir(), "kluctl-e2e-kubeconfig.yml")
	}
	return createKindCluster(kindClusterName, kindKubeconfig)
}

var defaultKindCluster = createDefaultKindCluster()
