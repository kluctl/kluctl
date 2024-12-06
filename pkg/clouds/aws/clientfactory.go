package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type GetSecretValueInterface interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type AwsClientFactory interface {
	SecretsManagerClient(ctx context.Context, profile *string, region *string) (GetSecretValueInterface, error)
}

type awsClientFactory struct {
	client    client.Client
	awsConfig *types.AwsConfig
}

func (a *awsClientFactory) SecretsManagerClient(ctx context.Context, profile *string, region *string) (GetSecretValueInterface, error) {
	var configOpts []func(*config.LoadOptions) error

	if region != nil {
		configOpts = append(configOpts, config.WithRegion(*region))
	}

	cfg, err := LoadAwsConfigHelper(ctx, a.client, a.awsConfig, profile, configOpts...)
	if err != nil {
		return nil, err
	}
	return secretsmanager.NewFromConfig(cfg), nil
}

func NewClientFactory(c client.Client, awsConfig *types.AwsConfig) AwsClientFactory {
	return &awsClientFactory{
		client:    c,
		awsConfig: awsConfig,
	}
}
