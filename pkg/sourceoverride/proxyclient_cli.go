package sourceoverride

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/kluctl/kluctl/lib/git"
	"github.com/kluctl/kluctl/lib/git/types"
	"github.com/kluctl/kluctl/lib/status"
	kluctlv1 "github.com/kluctl/kluctl/v2/api/v1beta1"
	oci_client "github.com/kluctl/kluctl/v2/pkg/oci/client"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"io"
	v1 "k8s.io/api/core/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/portforward"
	"k8s.io/client-go/transport/spdy"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProxyClientCli struct {
	ctx                 context.Context
	client              client.Client
	controllerNamespace string

	resolver Resolver

	key         *ecdsa.PrivateKey
	pubKeyBytes []byte
	pubKeyHash  []byte

	pod     *v1.Pod
	podPort int
	podFW   *portforward.PortForwarder

	target string

	grpcConn *grpc.ClientConn
	stream   Proxy_ProxyStreamClient
	smClient ProxyClient
}

func NewClientCli(ctx context.Context, c client.Client, controllerNamespace string, resolver Resolver) (*ProxyClientCli, error) {
	s := &ProxyClientCli{
		ctx:                 ctx,
		client:              c,
		controllerNamespace: controllerNamespace,
		resolver:            resolver,
	}

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	s.key = key

	pubKeyBytes, err := x509.MarshalPKIXPublicKey(key.Public())
	if err != nil {
		return nil, err
	}
	s.pubKeyBytes = pubKeyBytes

	h := sha256.New()
	h.Write(pubKeyBytes)
	s.pubKeyHash = h.Sum(nil)

	return s, nil
}

func (c *ProxyClientCli) BuildProxyUrl() (*url.URL, error) {
	var host string
	if c.pod != nil {
		host = fmt.Sprintf("%s:%d", c.pod.Status.PodIP, c.podPort)
	} else {
		host = c.target
	}
	u := fmt.Sprintf("%s://%s/%s", kluctlv1.SourceOverrideScheme, host, hex.EncodeToString(c.pubKeyHash))
	return url.Parse(u)
}

func (c *ProxyClientCli) ConnectToPod(restConfig *rest.Config, pod v1.Pod) error {
	var err error
	c.podPort, err = c.findPodPort(pod, "source-override")
	if err != nil {
		return err
	}

	fw, err := c.newPodPortForward(restConfig, pod, c.podPort)
	if err != nil {
		return err
	}
	c.podFW = fw
	c.pod = &pod

	ports, err := fw.GetPorts()
	if err != nil {
		return err
	}

	return c.Connect(fmt.Sprintf("localhost:%d", ports[0].Local))
}

func (c *ProxyClientCli) Connect(target string) error {
	cp, err := LoadTLSCA(c.ctx, c.client, c.controllerNamespace)
	if err != nil {
		return err
	}
	creds := credentials.NewClientTLSFromCert(cp, "")

	grpcConn, err := grpc.DialContext(c.ctx, target,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(creds),
	)
	if err != nil {
		return err
	}
	c.grpcConn = grpcConn

	c.smClient = NewProxyClient(c.grpcConn)
	c.target = target

	return nil
}

func (c *ProxyClientCli) findPodPort(pod v1.Pod, portName string) (int, error) {
	for _, c := range pod.Spec.Containers {
		for _, p := range c.Ports {
			if p.Name == portName {
				return int(p.ContainerPort), nil
			}
		}
	}
	return 0, fmt.Errorf("pod does not have containerPort with name %s", portName)
}

func (c *ProxyClientCli) newPodPortForward(restConfig *rest.Config, pod v1.Pod, podPort int) (*portforward.PortForwarder, error) {
	cv1Client, err := corev1.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	req := cv1Client.RESTClient().Post().
		Resource("pods").
		Namespace(pod.Namespace).
		Name(pod.Name).
		SubResource("portforward")

	transport, upgrader, err := spdy.RoundTripperFor(restConfig)
	if err != nil {
		return nil, err
	}
	dialer := spdy.NewDialer(upgrader, &http.Client{Transport: transport}, "POST", req.URL())

	readyCh := make(chan struct{})
	fw, err := portforward.New(dialer, []string{fmt.Sprintf(":%d", podPort)}, c.ctx.Done(), readyCh, nil, nil)
	if err != nil {
		return nil, err
	}

	go func() {
		_ = fw.ForwardPorts()
	}()

	<-readyCh

	return fw, err
}

func (c *ProxyClientCli) Handshake() error {
	var err error
	c.stream, err = c.smClient.ProxyStream(c.ctx)
	if err != nil {
		return err
	}

	msg := &ProxyResponse{
		Auth: &AuthMsg{
			PubKey: c.pubKeyBytes,
		},
	}

	err = c.stream.Send(msg)
	if err != nil {
		return err
	}

	resp, err := c.stream.Recv()
	if err != nil {
		return err
	}

	if resp.Auth == nil || len(resp.Auth.Challenge) != challengeSize {
		return fmt.Errorf("response is missing auth message")
	}

	h := sha256.New()
	h.Write(c.pubKeyHash)
	h.Write(resp.Auth.Challenge)

	sig, err := ecdsa.SignASN1(rand.Reader, c.key, h.Sum(nil))
	if err != nil {
		return err
	}

	msg = &ProxyResponse{
		Auth: &AuthMsg{
			Pop: sig,
		},
	}
	err = c.stream.Send(msg)
	if err != nil {
		return err
	}
	resp, err = c.stream.Recv()
	if err != nil {
		return err
	}

	if resp.Auth == nil {
		return fmt.Errorf("response is missing auth message")
	}

	if resp.Auth.AuthError == nil {
		return fmt.Errorf("missing authError")
	}
	if *resp.Auth.AuthError != "" {
		return errors.New(*resp.Auth.AuthError)
	}
	return nil
}

func (c *ProxyClientCli) Start() error {
	for {
		req, err := c.stream.Recv()
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if req.Request == nil {
			return fmt.Errorf("missing request")
		}
		var resp ResolveOverrideResponse
		b, err := c.handleRequest(req.Request)
		if err != nil {
			resp.Error = utils.Ptr(err.Error())
		} else {
			resp.Artifact = b
		}

		err = c.stream.Send(&ProxyResponse{
			Response: &resp,
		})
		if err != nil {
			return err
		}
	}
}

func (c *ProxyClientCli) handleRequest(req *ResolveOverrideRequest) ([]byte, error) {
	ctx := c.stream.Context()

	repoKey, err := types.ParseRepoKey(req.RepoKey, "")
	if err != nil {
		return nil, fmt.Errorf("invalid RepoKey: %w", err)
	}
	if repoKey.Type == "" {
		return nil, fmt.Errorf("missing type in RepoKey")
	}

	p, err := c.resolver.ResolveOverride(ctx, repoKey)
	if err != nil {
		return nil, fmt.Errorf("ResolveOverride failed: %w", err)
	}
	if p == "" {
		return nil, nil
	}

	status.Infof(c.ctx, "Controller requested override for %s", req.RepoKey)

	f, err := os.CreateTemp(utils.GetTmpBaseDir(ctx), "so-*.tgz")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	f.Close()
	defer os.Remove(f.Name())

	absPath, err := filepath.Abs(p)
	if err != nil {
		return nil, err
	}
	ignorePatterns, err := git.LoadGitignore(absPath)
	if err != nil {
		return nil, err
	}

	ociClient := oci_client.NewClient(nil)
	err = ociClient.Build(f.Name(), p, ignorePatterns)
	if err != nil {
		return nil, fmt.Errorf("failed to build artifact: %w", err)
	}

	st, err := os.Stat(f.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to stat artifact: %w", err)
	}
	if st.Size() > maxTarSize {
		return nil, fmt.Errorf("tgz is too large: %d", st.Size())
	}

	b, err := os.ReadFile(f.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to read artifact: %w", err)
	}

	status.Infof(c.ctx, "Sending source override tarball with %d bytes", len(b))

	return b, nil
}
