package sops

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"go.mozilla.org/sops/v3/kms"
	"sigs.k8s.io/yaml"
)

// LoadCredsProviderFromYaml parses the given YAML returns a CredsProvider object
// which contains the credentials provider used for authenticating towards AWS KMS.
func LoadCredsProviderFromYaml(b []byte) (*kms.CredentialsProvider, error) {
	credInfo := struct {
		AccessKeyID     string `json:"aws_access_key_id"`
		SecretAccessKey string `json:"aws_secret_access_key"`
		SessionToken    string `json:"aws_session_token"`
	}{}
	if err := yaml.Unmarshal(b, &credInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal AWS credentials file: %w", err)
	}
	cp := kms.NewCredentialsProvider(credentials.NewStaticCredentialsProvider(credInfo.AccessKeyID, credInfo.SecretAccessKey, credInfo.SessionToken))

	return cp, nil
}
