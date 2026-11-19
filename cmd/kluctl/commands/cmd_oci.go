package commands

import (
	"context"

	"github.com/fluxcd/pkg/oci"
	"github.com/kluctl/kluctl/v2/cmd/kluctl/args"
	"github.com/kluctl/kluctl/v2/pkg/oci/auth_provider"
)

type ociCmd struct {
	Push ociPushCmd `cmd:"" help:"Push to an oci repository"`
	Pull ociPullCmd `cmd:"" help:"Pull from an OCI repository"`
}

func newOCIClient(ctx context.Context, url string, credentials *args.RegistryCredentials) (*oci.Client, error) {
	providers := auth_provider.NewDefaultAuthProviders("KLUCTL_REGISTRY")
	provider, err := credentials.BuildAuthProvider(ctx)
	if err != nil {
		return nil, err
	}
	providers.RegisterAuthProvider(provider, false)

	auth, err := providers.FindAuthEntry(ctx, url)
	if err != nil {
		return nil, err
	}
	opts, err := auth.BuildCraneOptions()
	if err != nil {
		return nil, err
	}
	return oci.NewClient(append(oci.DefaultOptions(), opts...)), nil
}
