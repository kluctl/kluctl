package deployment

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/git/repocache"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/vars"
)

type SharedContext struct {
	Ctx             context.Context
	K               *k8s.K8sCluster
	RP              *repocache.GitRepoCache
	VarsLoader      *vars.VarsLoader
	HelmCredentials HelmCredentialsProvider

	RenderDir                         string
	SealedSecretsDir                  string
	DefaultSealedSecretsOutputPattern string
}
