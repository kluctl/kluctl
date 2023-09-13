package gcp

import (
	"context"
	"fmt"

	"github.com/googleapis/gax-go/v2"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FakeClientFactory struct {
	Secrets map[string]string
}

func (f *FakeClientFactory) AccessSecretVersion(ctx context.Context, req *secretmanagerpb.AccessSecretVersionRequest, opts ...gax.CallOption) (*secretmanagerpb.AccessSecretVersionResponse, error) {
	if secret, ok := f.Secrets[req.Name]; ok {
		return &secretmanagerpb.AccessSecretVersionResponse{
			Payload: &secretmanagerpb.SecretPayload{
				Data: []byte(secret),
			},
		}, nil
	}

	errMsg := fmt.Sprintf("secret not found: failed to access secret version: rpc error: code = NotFound desc = secret %s not found", req.Name)
	return nil, status.Errorf(codes.NotFound, errMsg)
}

func (f *FakeClientFactory) SecretManagerClient(ctx context.Context) (AccessSecretVersionInterface, error) {
	return f, nil
}

func NewFakeClientFactory() *FakeClientFactory {
	return &FakeClientFactory{
		Secrets: make(map[string]string),
	}
}
