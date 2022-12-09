package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
)

type FakeAwsClientFactory struct {
	secretsmanageriface.SecretsManagerAPI

	Secrets map[string]string
}

func (f *FakeAwsClientFactory) GetSecretValue(in *secretsmanager.GetSecretValueInput) (*secretsmanager.GetSecretValueOutput, error) {
	name := *in.SecretId
	arn, err := ParseArn(*in.SecretId)
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

	return nil, awserr.New(secretsmanager.ErrCodeResourceNotFoundException, fmt.Sprintf("secret %s not found", *in.SecretId), nil)
}

func (f *FakeAwsClientFactory) SecretsManagerClient(profile *string, region *string) (secretsmanageriface.SecretsManagerAPI, error) {
	return f, nil
}

func NewFakeClientFactory() *FakeAwsClientFactory {
	return &FakeAwsClientFactory{}
}
