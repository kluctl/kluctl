package sourceoverride

import (
	"bytes"
	"context"
	"errors"
	"github.com/kluctl/kluctl/v2/pkg/tar"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"os"
)

type ProxyClientController struct {
	serverId string
	repoKey  types.RepoKey
	isGroup  bool

	grpcConn *grpc.ClientConn
	smClient ProxyClient

	cache utils.ThreadSafeCache[types.RepoKey, string]
}

func NewClientController(serverId string, repoKey types.RepoKey, isGroup bool) (*ProxyClientController, error) {
	s := &ProxyClientController{
		serverId: serverId,
		repoKey:  repoKey,
		isGroup:  isGroup,
	}
	return s, nil
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
	grpcConn, err := grpc.DialContext(ctx, target,
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)),
	)
	if err != nil {
		return err
	}
	c.grpcConn = grpcConn
	c.smClient = NewProxyClient(c.grpcConn)
	return nil
}

func (c *ProxyClientController) ResolveOverride(ctx context.Context, repoKey types.RepoKey) (string, error) {
	tmp := RepoOverride{
		RepoKey: c.repoKey,
		IsGroup: c.isGroup,
	}
	if _, ok := tmp.Match(repoKey); !ok {
		return "", nil
	}

	return c.cache.Get(repoKey, func() (string, error) {
		return c.doResolveOverride(ctx, repoKey)
	})
}

func (c *ProxyClientController) doResolveOverride(ctx context.Context, repoKey types.RepoKey) (string, error) {
	msg := &ProxyRequest{
		ServerId: c.serverId,
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
