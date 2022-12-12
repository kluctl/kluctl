package deployment

import (
	"context"
	"github.com/kluctl/kluctl/v2/pkg/helm"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/sops"
	"github.com/kluctl/kluctl/v2/pkg/vars"
)

type SharedContext struct {
	Ctx             context.Context
	K               *k8s.K8sCluster
	K8sVersion      string
	RP              *repocache.GitRepoCache
	SopsDecrypter   sops.SopsDecrypter
	VarsLoader      *vars.VarsLoader
	HelmCredentials helm.HelmCredentialsProvider

	RenderDir                         string
	SealedSecretsDir                  string
	DefaultSealedSecretsOutputPattern string
	HelmChartsDir                     string
}
