package aws

import (
	"context"
	"fmt"
	arn2 "github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

type FakeAwsClientFactory struct {
	GetSecretValueInterface

	Secrets map[string]string
}

func (f *FakeAwsClientFactory) GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error) {
	var arnResource Resource
	arn, err := arn2.Parse(*params.SecretId)
	if err == nil {
		arnResource, _ = SplitResource(arn.Resource)
	} else {
		arnResource, _ = SplitResource(*params.SecretId)
	}

	s, ok := f.Secrets[arnResource.ResourceId]
	if ok {
		return &secretsmanager.GetSecretValueOutput{
			Name:         &arnResource.ResourceId,
			SecretString: &s,
		}, nil
	}

	errMsg := fmt.Sprintf("secret %s not found", *params.SecretId)
	return nil, &types.ResourceNotFoundException{
		Message: &errMsg,
	}
}

func (f *FakeAwsClientFactory) SecretsManagerClient(ctx context.Context, profile *string, region *string) (GetSecretValueInterface, error) {
	return f, nil
}

func NewFakeClientFactory() *FakeAwsClientFactory {
	return &FakeAwsClientFactory{}
}
