package gcp

import (
	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"github.com/googleapis/gax-go/v2"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"

	"context"
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
