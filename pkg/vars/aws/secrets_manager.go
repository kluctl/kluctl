package aws

import (
	"fmt"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

func GetAwsSecretsManagerSecret(aws AwsClientFactory, profile *string, region *string, secretName string) (string, error) {
	if region == nil {
		arn, err := ParseArn(secretName)
		if err != nil {
			return "", fmt.Errorf("when omitting the AWS region, the secret name must be a valid ARN")
		}
		region = &arn.Region
	}

	smClient, err := aws.SecretsManagerClient(profile, region)
	if err != nil {
		return "", fmt.Errorf("getting secret %s from AWS secrets manager failed: %w", secretName, err)
	}

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
