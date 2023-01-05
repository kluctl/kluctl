package aws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func GetAwsSecretsManagerSecret(ctx context.Context, aws AwsClientFactory, profile *string, region *string, secretName string) (string, error) {
	if !arn.IsARN(secretName) {
		return "", fmt.Errorf("%s is not a valid ARN.", secretName)
	}
	if region == nil {
		arn, err := arn.Parse(secretName)
		if err != nil {
			return "", fmt.Errorf("Can't parse the ARN %s", secretName)
		}
		}
		region = &arn.Region
	}

	smClient, err := aws.SecretsManagerClient(profile, region)
	if err != nil {
		return "", fmt.Errorf("getting secret %s from AWS secrets manager failed: %w", secretName, err)
	}

	r, err := smClient.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
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
