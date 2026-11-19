package gcp

import (
	"context"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"github.com/googleapis/gax-go/v2"
)

type AccessSecretVersionInterface interface {
	AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error)
}

type GcpClientFactory interface {
	SecretManagerClient(ctx context.Context) (AccessSecretVersionInterface, error)
}

type gcpClientFactory struct {
}

func (g *gcpClientFactory) SecretManagerClient(ctx context.Context) (AccessSecretVersionInterface, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func NewClientFactory() GcpClientFactory {
	return &gcpClientFactory{}
}
