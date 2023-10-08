/*
Copyright 2022 The Flux authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package login

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/fluxcd/pkg/oci"
	"github.com/fluxcd/pkg/oci/auth/aws"
	"github.com/fluxcd/pkg/oci/auth/azure"
	"github.com/fluxcd/pkg/oci/auth/gcp"
)

// ImageRegistryProvider analyzes the provided registry and returns the identified
// container image registry provider.
func ImageRegistryProvider(url string, ref name.Reference) oci.Provider {
	// If the url is a repository root address, use it to analyze. Else, derive
	// the registry from the name reference.
	// NOTE: This is because name.Reference of a repository root assumes that
	// the reference is an image name and defaults to using index.docker.io as
	// the registry host.
	addr := strings.TrimSuffix(url, "/")
	if strings.ContainsRune(addr, '/') {
		addr = ref.Context().RegistryStr()
	}

	_, _, ok := aws.ParseRegistry(addr)
	if ok {
		return oci.ProviderAWS
	}
	if gcp.ValidHost(addr) {
		return oci.ProviderGCP
	}
	if azure.ValidHost(addr) {
		return oci.ProviderAzure
	}
	return oci.ProviderGeneric
}

// ProviderOptions contains options for registry provider login.
type ProviderOptions struct {
	// AwsAutoLogin enables automatic attempt to get credentials for images in
	// ECR.
	AwsAutoLogin bool
	// GcpAutoLogin enables automatic attempt to get credentials for images in
	// GCP.
	GcpAutoLogin bool
	// AzureAutoLogin enables automatic attempt to get credentials for images in
	// ACR.
	AzureAutoLogin bool
}

// Manager is a login manager for various registry providers.
type Manager struct {
	ecr *aws.Client
	gcr *gcp.Client
	acr *azure.Client
}

// NewManager initializes a Manager with default registry clients
// configurations.
func NewManager() *Manager {
	return &Manager{
		ecr: aws.NewClient(),
		gcr: gcp.NewClient(),
		acr: azure.NewClient(),
	}
}

// WithECRClient allows overriding the default ECR client.
func (m *Manager) WithECRClient(c *aws.Client) *Manager {
	m.ecr = c
	return m
}

// WithGCRClient allows overriding the default GCR client.
func (m *Manager) WithGCRClient(c *gcp.Client) *Manager {
	m.gcr = c
	return m
}

// WithACRClient allows overriding the default ACR client.
func (m *Manager) WithACRClient(c *azure.Client) *Manager {
	m.acr = c
	return m
}

// Login performs authentication against a registry and returns the Authenticator.
// For generic registry provider, it is no-op.
func (m *Manager) Login(ctx context.Context, url string, ref name.Reference, opts ProviderOptions) (authn.Authenticator, error) {
	switch ImageRegistryProvider(url, ref) {
	case oci.ProviderAWS:
		return m.ecr.Login(ctx, opts.AwsAutoLogin, url)
	case oci.ProviderGCP:
		return m.gcr.Login(ctx, opts.GcpAutoLogin, url, ref)
	case oci.ProviderAzure:
		return m.acr.Login(ctx, opts.AzureAutoLogin, url, ref)
	}
	return nil, nil
}

// OIDCLogin attempts to get an Authenticator for the provided URL endpoint.
//
// If you want to construct an Authenticator based on an image reference,
// you may want to use Login instead.
func (m *Manager) OIDCLogin(ctx context.Context, registryURL string, opts ProviderOptions) (authn.Authenticator, error) {
	u, err := url.Parse(registryURL)
	if err != nil {
		return nil, fmt.Errorf("unable to parse registry url: %w", err)
	}

	provider := ImageRegistryProvider(u.Host, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to set up provider: %w", err)
	}

	switch provider {
	case oci.ProviderAWS:
		if !opts.AwsAutoLogin {
			return nil, fmt.Errorf("ECR authentication failed: %w", oci.ErrUnconfiguredProvider)
		}
		log.FromContext(ctx).Info("logging in to AWS ECR for " + u.Host)
		return m.ecr.OIDCLogin(ctx, u.Host)
	case oci.ProviderGCP:
		if !opts.GcpAutoLogin {
			return nil, fmt.Errorf("GCR authentication failed: %w", oci.ErrUnconfiguredProvider)
		}
		log.FromContext(ctx).Info("logging in to GCP GCR for " + u.Host)
		return m.gcr.OIDCLogin(ctx)
	case oci.ProviderAzure:
		if !opts.AzureAutoLogin {
			return nil, fmt.Errorf("ACR authentication failed: %w", oci.ErrUnconfiguredProvider)
		}
		log.FromContext(ctx).Info("logging in to Azure ACR for " + u.Host)
		return m.acr.OIDCLogin(ctx, fmt.Sprintf("%s://%s", u.Scheme, u.Host))
	}
	return nil, nil
}
