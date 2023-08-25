package sops

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/arn"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/logging"
	"github.com/getsops/sops/v3/keyservice"
	"github.com/getsops/sops/v3/kms"
	intkeyservice "github.com/kluctl/kluctl/v2/pkg/controllers/internal/sops/keyservice"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"os"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func BuildSopsKeyServerFromServiceAccount(ctx context.Context, client client.Client, sa *corev1.ServiceAccount) (keyservice.KeyServiceClient, error) {
	var serverOpts []intkeyservice.ServerOption

	x, err := withAWSWebIdentity(ctx, client, sa)
	if err != nil {
		return nil, err
	}
	serverOpts = append(serverOpts, x...)

	server := intkeyservice.NewServer(serverOpts...)
	return keyservice.NewCustomLocalClient(server), nil
}

type webIdentityToken string

func (t webIdentityToken) GetIdentityToken() ([]byte, error) {
	return []byte(t), nil
}

func withAWSWebIdentity(ctx context.Context, client client.Client, sa *corev1.ServiceAccount) ([]intkeyservice.ServerOption, error) {
	roleArnStr := ""
	if sa.GetAnnotations() != nil {
		roleArnStr, _ = sa.GetAnnotations()["eks.amazonaws.com/role-arn"]
	}
	if roleArnStr == "" {
		return nil, nil
	}
	roleArn, err := arn.Parse(roleArnStr)
	if err != nil {
		return nil, err
	}

	exp := int64(60 * 10)

	tokenRequest := authenticationv1.TokenRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      sa.Name,
			Namespace: sa.Namespace,
		},
		Spec: authenticationv1.TokenRequestSpec{
			Audiences:         []string{"sts.amazonaws.com"},
			ExpirationSeconds: &exp,
		},
	}

	err = client.SubResource("token").Create(ctx, sa, &tokenRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to create token for AWS STS: %w", err)
	}

	cfg := aws.Config{
		Credentials: aws.AnonymousCredentials{},
		Logger:      logging.NewStandardLogger(os.Stderr),
		Region:      roleArn.Region,
	}
	if cfg.Region == "" {
		cfg.Region = "aws-global"
	}

	optFns := []func(*stscreds.WebIdentityRoleOptions){
		func(options *stscreds.WebIdentityRoleOptions) {
			options.RoleSessionName = "kluctl-sops-decrypter"
		},
	}

	provider := stscreds.NewWebIdentityRoleProvider(sts.NewFromConfig(cfg), roleArnStr, webIdentityToken(tokenRequest.Status.Token), optFns...)

	var serverOpts []intkeyservice.ServerOption
	serverOpts = append(serverOpts, intkeyservice.WithAWSKeys{CredsProvider: kms.NewCredentialsProvider(provider)})
	return serverOpts, nil
}
