package aws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

type FakeAwsClientFactory struct {
	GetSecretValueInterface

	Secrets map[string]string
}

func (f *FakeAwsClientFactory) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	arn, err := arn.Parse(*params.SecretId)
	if err == nil {
		arnString := arn.String()
		s, ok := f.Secrets[arnString]
		if ok {
			return &secretsmanager.GetSecretValueOutput{
				Name:         &arnString,
				SecretString: &s,
			}, nil
		}
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
