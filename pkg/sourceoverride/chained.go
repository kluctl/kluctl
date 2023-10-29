package sourceoverride

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/types"
)

type ChainedResolver struct {
	resolvers []Resolver
}

func NewChainedResolver(resolvers []Resolver) *ChainedResolver {
	return &ChainedResolver{
		resolvers: resolvers,
	}
}

func (c *ChainedResolver) ResolveOverride(ctx context.Context, repoKey types.RepoKey) (string, error) {
	for _, x := range c.resolvers {
		p, err := x.ResolveOverride(ctx, repoKey)
		if err != nil {
			return "", err
		}
		if p != "" {
			return p, nil
		}
	}
	return "", nil
}
