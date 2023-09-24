package kluctl_project

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/getsops/sops/v3/keyservice"
	"github.com/getsops/sops/v3/kms"
	"github.com/kluctl/kluctl/v2/pkg/sops"
	"github.com/kluctl/kluctl/v2/pkg/sops/decryptor"
	intkeyservice "github.com/kluctl/kluctl/v2/pkg/sops/keyservice"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (c *LoadedKluctlProject) buildSopsDecrypter(ctx context.Context, client client.Client, target *types.Target) (*decryptor.Decryptor, error) {
	d := decryptor.NewDecryptor(c.LoadArgs.ProjectDir, decryptor.MaxEncryptedFileSize)

	err := c.addAwsKeyServers(ctx, client, d, target)
	if err != nil {
		return nil, err
	}

	if c.LoadArgs.AddKeyServersFunc != nil {
		err = c.LoadArgs.AddKeyServersFunc(ctx, d)
		if err != nil {
			return nil, err
		}
	} else {
		d.AddLocalKeyService()
	}

	return d, nil
}

func (c *LoadedKluctlProject) addAwsKeyServers(ctx context.Context, client client.Client, d *decryptor.Decryptor, target *types.Target) error {
	if target.Aws == nil {
		return nil
	}

	if target.Aws.Profile != nil {
		var configOpts []func(*config.LoadOptions) error
		configOpts = append(configOpts, config.WithSharedConfigProfile(*target.Aws.Profile))
		cfg, err := config.LoadDefaultConfig(context.Background(), configOpts...)
		if err == nil {
			server := intkeyservice.NewServer(intkeyservice.WithAWSKeys{CredsProvider: kms.NewCredentialsProvider(cfg.Credentials)})
			ks := keyservice.NewCustomLocalClient(server)
			d.AddKeyServiceClient(ks)
		}
	}

	if target.Aws.ServiceAccount != nil {
		ks, err := sops.BuildSopsKeyServerFromServiceAccount(ctx, client, target.Aws.ServiceAccount.Name, target.Aws.ServiceAccount.Namespace)
		if err != nil {
			return err
		}
		if ks != nil {
			d.AddKeyServiceClient(ks)
		}
	}

	return nil
}
