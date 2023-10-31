package target_context

import (
	"context"
	"github.com/getsops/sops/v3/keyservice"
	"github.com/getsops/sops/v3/kms"
	"github.com/kluctl/kluctl/v2/pkg/clouds/aws"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	intkeyservice "github.com/kluctl/kluctl/v2/pkg/sops/keyservice"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func buildSopsDecrypter(ctx context.Context, rootDir string, client client.Client, target *types.Target, addKeyServersFunc func(ctx context.Context, d *decryptor.Decryptor) error) (*decryptor.Decryptor, error) {
	d := decryptor.NewDecryptor(rootDir, decryptor.MaxEncryptedFileSize)

	err := addAwsKeyServers(ctx, client, d, target)
	if err != nil {
		return nil, err
	}

	if addKeyServersFunc != nil {
		err = addKeyServersFunc(ctx, d)
		if err != nil {
			return nil, err
		}
	} else {
		d.AddLocalKeyService()
	}

	return d, nil
}

func addAwsKeyServers(ctx context.Context, client client.Client, d *decryptor.Decryptor, target *types.Target) error {
	cfg, err := aws.LoadAwsConfigHelper(ctx, client, target.Aws, nil)
	if err != nil {
		return err
	}

	server := intkeyservice.NewServer(intkeyservice.WithAWSKeys{CredsProvider: kms.NewCredentialsProvider(cfg.Credentials)})
	ks := keyservice.NewCustomLocalClient(server)
	d.AddKeyServiceClient(ks)

	return nil
}
