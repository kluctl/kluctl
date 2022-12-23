package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

type GetSecretValueInterface interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

type AwsClientFactory interface {
	SecretsManagerClient(profile *string, region *string) (GetSecretValueInterface, error)
}

type awsClientFactory struct {
}

func (a *awsClientFactory) SecretsManagerClient(profile *string, region *string) (GetSecretValueInterface, error) {
	var configOpts []func(*config.LoadOptions) error
	if profile != nil {
		configOpts = append(configOpts, config.WithSharedConfigProfile(*profile))
	}

	if region != nil {
		configOpts = append(configOpts, config.WithRegion(*region))
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), configOpts...)
	if err != nil {
		return nil, err
	}

	return secretsmanager.NewFromConfig(cfg), nil
}

func NewClientFactory() AwsClientFactory {
	return &awsClientFactory{}
}
