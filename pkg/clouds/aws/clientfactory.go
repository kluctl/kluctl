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
	if profile == nil {
		profile = a.awsConfig.Profile
	}

	var configOpts []func(*config.LoadOptions) error
	if profile != nil {
		configOpts = append(configOpts, config.WithSharedConfigProfile(*profile))
	}

	if region != nil {
		configOpts = append(configOpts, config.WithRegion(*region))
	}

	if a.awsConfig.ServiceAccount != nil {
		webIdentityCreds, err := BuildCredentialsFromServiceAccount(ctx, a.client, a.awsConfig.ServiceAccount.Name, a.awsConfig.ServiceAccount.Namespace, "kluctl-vars-loader")
		if err == nil && webIdentityCreds != nil {
			configOpts = append(configOpts, config.WithCredentialsProvider(webIdentityCreds))
		}
	}

	cfg, err := config.LoadDefaultConfig(context.Background(), configOpts...)
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
