package sourceoverride

import (
	"bytes"
	"context"
	"errors"
	"github.com/kluctl/kluctl/v2/pkg/tar"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ProxyClientController struct {
	client              client.Reader
	controllerNamespace string

	pubKeyHash     string
	knownOverrides []RepoOverride

	grpcConn         *grpc.ClientConn
	smClient         ProxyClient
	controllerSecret []byte

	cache utils.ThreadSafeCache[types.RepoKey, string]
}

func NewClientController(c client.Reader, controllerNamespace string, pubKeyHash string) (*ProxyClientController, error) {
	s := &ProxyClientController{
		client:              c,
		controllerNamespace: controllerNamespace,
		pubKeyHash:          pubKeyHash,
	}

	return s, nil
}

func (c *ProxyClientController) AddKnownOverride(ro RepoOverride) {
	c.knownOverrides = append(c.knownOverrides, ro)
}

func (c *ProxyClientController) Close() error {
	return c.grpcConn.Close()
}

func (c *ProxyClientController) Cleanup() {
	c.cache.ForEach(func(k types.RepoKey, v string) {
		_ = os.RemoveAll(v)
	})
}

func (c *ProxyClientController) Connect(ctx context.Context, target string) error {
	cp, _, controllerSecret, err := WaitAndLoadSecret(ctx, c.client, c.controllerNamespace)
	if err != nil {
		return err
	}

	creds := credentials.NewClientTLSFromCert(cp, "")

	grpcConn, err := grpc.DialContext(ctx, target,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(creds),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)),
		grpc.WithAuthority("source-override"),
	)
	if err != nil {
		return err
	}
	c.grpcConn = grpcConn
	c.smClient = NewProxyClient(c.grpcConn)
	c.controllerSecret = controllerSecret
	return nil
}

func (c *ProxyClientController) ResolveOverride(ctx context.Context, repoKey types.RepoKey) (string, error) {
	anyMatch := false
	for _, ro := range c.knownOverrides {
		if _, ok := ro.Match(repoKey); ok {
			anyMatch = true
			break
		}
	}
	if !anyMatch {
		return "", nil
	}

	return c.cache.Get(repoKey, func() (string, error) {
		return c.doResolveOverride(ctx, repoKey)
	})
}

func (c *ProxyClientController) doResolveOverride(ctx context.Context, repoKey types.RepoKey) (string, error) {
	msg := &ProxyRequest{
		Auth: &AuthMsg{
			PubKeyHash:       &c.pubKeyHash,
			ControllerSecret: c.controllerSecret,
		},
		Request: &ResolveOverrideRequest{
			RepoKey: repoKey.String(),
		},
	}
	resp, err := c.smClient.ResolveOverride(ctx, msg)
	if err != nil {
		return "", err
	}
	if resp.Error != nil {
		return "", errors.New(*resp.Error)
	}
	if resp.Artifact == nil {
		return "", nil
	}

	cleanup := true
	dir, err := os.MkdirTemp(utils.GetTmpBaseDir(ctx), "source-override-*")
	defer func() {
		if cleanup {
			_ = os.RemoveAll(dir)
		}
	}()

	err = tar.Untar(bytes.NewReader(resp.Artifact), dir, tar.WithMaxUntarSize(maxUntarSize), tar.WithSkipSymlinks())
	if err != nil {
		return "", err
	}

	cleanup = false
	return dir, nil
}
