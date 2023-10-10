package deployment

import (
	"context"
	helm_auth "github.com/kluctl/kluctl/v2/pkg/helm/auth"
	"github.com/kluctl/kluctl/v2/pkg/k8s"
	"github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
	"github.com/kluctl/kluctl/v2/pkg/repocache"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	"github.com/kluctl/kluctl/v2/pkg/vars"
)

type SharedContext struct {
	Ctx              context.Context
	K                *k8s.K8sCluster
	K8sVersion       string
	GitRP            *repocache.GitRepoCache
	OciRP            *repocache.OciRepoCache
	SopsDecrypter    *decryptor.Decryptor
	VarsLoader       *vars.VarsLoader
	HelmAuthProvider helm_auth.HelmAuthProvider
	OciAuthProvider  auth_provider.OciAuthProvider

	Discriminator                     string
	RenderDir                         string
	SealedSecretsDir                  string
	DefaultSealedSecretsOutputPattern string
}
