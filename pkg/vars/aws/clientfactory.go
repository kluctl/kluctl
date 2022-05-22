package aws

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/aws/aws-sdk-go/service/secretsmanager/secretsmanageriface"
	"os"
)

type AwsClientFactory interface {
	SecretsManagerClient(profile *string, region *string) (secretsmanageriface.SecretsManagerAPI, error)
}

type awsClientFactory struct {
}

func (a *awsClientFactory) getSession(profile *string) (*session.Session, error) {
	var opts session.Options
	opts.SharedConfigState = session.SharedConfigEnable
	// Environment variable always takes precedence
	if _, ok := os.LookupEnv("AWS_PROFILE"); !ok && profile != nil {
		opts.Profile = *profile
	}
	s, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (a *awsClientFactory) SecretsManagerClient(profile *string, region *string) (secretsmanageriface.SecretsManagerAPI, error) {
	s, err := a.getSession(profile)
	if err != nil {
		return nil, err
	}

	return secretsmanager.New(s, &aws.Config{Region: region}), nil
}

func NewClientFactory() AwsClientFactory {
	return &awsClientFactory{}
}
