package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"

	"os"
)

func GetAwsSecretsManagerSecret(profile *string, region *string, secretName string) (string, error) {
	var opts session.Options
	opts.SharedConfigState = session.SharedConfigEnable
	// Environment variable always takes precedence
	if _, ok := os.LookupEnv("AWS_PROFILE"); !ok && region != nil {
		opts.Profile = *profile
	}
	s, err := session.NewSessionWithOptions(opts)
	if err != nil {
		return "", err
	}

	if region == nil {
		arn, err := ParseArn(secretName)
		if err != nil {
			return "", fmt.Errorf("when omitting the AWS region, the secret name must be a valid ARN")
		}
		region = &arn.Region
	}

	smClient := secretsmanager.New(s, &aws.Config{Region: region})
	r, err := smClient.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: &secretName,
	})
	if err != nil {
		return "", fmt.Errorf("getting secret %s from AWS secrets manager failed: %w", secretName, err)
	}

	var secret string
	if r.SecretString != nil {
		secret = *r.SecretString
	} else {
		secret = string(r.SecretBinary)
	}

	return secret, nil
}
