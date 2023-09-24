package sops

import (
	"context"
	"github.com/getsops/sops/v3/keyservice"
	"github.com/getsops/sops/v3/kms"
	"github.com/kluctl/kluctl/v2/pkg/clouds/aws"
	intkeyservice "github.com/kluctl/kluctl/v2/pkg/controllers/internal/sops/keyservice"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func BuildSopsKeyServerFromServiceAccount(ctx context.Context, client client.Client, name string, namespace string) (keyservice.KeyServiceClient, error) {
	var serverOpts []intkeyservice.ServerOption

	provider, err := aws.BuildCredentialsFromServiceAccount(ctx, client, name, namespace, "kluctl-sops-decrypter")
	if err != nil || provider == nil {
		return nil, err
	}

	serverOpts = append(serverOpts, intkeyservice.WithAWSKeys{CredsProvider: kms.NewCredentialsProvider(provider)})

	server := intkeyservice.NewServer(serverOpts...)
	return keyservice.NewCustomLocalClient(server), nil
}
