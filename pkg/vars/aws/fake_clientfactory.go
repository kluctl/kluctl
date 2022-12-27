package aws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

type FakeAwsClientFactory struct {
	GetSecretValueInterface

	Secrets map[string]string
}

func (f *FakeAwsClientFactory) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	name := *params.SecretId
	arn, err := ParseArn(*params.SecretId)
	if err == nil {
		name = arn.Resource
	}

	s, ok := f.Secrets[name]
	if ok {
		return &secretsmanager.GetSecretValueOutput{
			Name:         &name,
			SecretString: &s,
		}, nil
	}

	errMsg := fmt.Sprintf("secret %s not found", *params.SecretId)
	return nil, &types.ResourceNotFoundException{
		Message: &errMsg,
	}
}

func (f *FakeAwsClientFactory) SecretsManagerClient(profile *string, region *string) (GetSecretValueInterface, error) {
	return f, nil
}

func NewFakeClientFactory() *FakeAwsClientFactory {
	return &FakeAwsClientFactory{}
}
