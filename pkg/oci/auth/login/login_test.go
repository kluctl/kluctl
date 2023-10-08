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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/google/go-containerregistry/pkg/name"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/oci"
	"github.com/fluxcd/pkg/oci/auth/aws"
	"github.com/fluxcd/pkg/oci/auth/azure"
	"github.com/fluxcd/pkg/oci/auth/gcp"
)

func TestImageRegistryProvider(t *testing.T) {
	tests := []struct {
		name  string
		image string
		want  oci.Provider
	}{
		{"ecr", "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo:v1", oci.ProviderAWS},
		{"ecr-root", "012345678901.dkr.ecr.us-east-1.amazonaws.com", oci.ProviderAWS},
		{"ecr-root with slash", "012345678901.dkr.ecr.us-east-1.amazonaws.com/", oci.ProviderAWS},
		{"gcr", "gcr.io/foo/bar:v1", oci.ProviderGCP},
		{"gcr-root", "gcr.io", oci.ProviderGCP},
		{"acr", "foo.azurecr.io/bar:v1", oci.ProviderAzure},
		{"acr-root", "foo.azurecr.io", oci.ProviderAzure},
		{"docker.io", "foo/bar:v1", oci.ProviderGeneric},
		{"docker.io-root", "docker.io", oci.ProviderGeneric},
		{"library", "alpine", oci.ProviderGeneric},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Trim suffix to allow parsing it as reference without modifying
			// the given image address.
			ref, err := name.ParseReference(strings.TrimSuffix(tt.image, "/"))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(ImageRegistryProvider(tt.image, ref)).To(Equal(tt.want))
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name         string
		responseBody string
		statusCode   int
		providerOpts ProviderOptions
		beforeFunc   func(serverURL string, mgr *Manager, image *string)
		wantErr      bool
	}{
		{
			name:         "ecr",
			responseBody: `{"authorizationData": [{"authorizationToken": "c29tZS1rZXk6c29tZS1zZWNyZXQ="}]}`,
			providerOpts: ProviderOptions{AwsAutoLogin: true},
			beforeFunc: func(serverURL string, mgr *Manager, image *string) {
				// Create ECR client and configure the manager.
				ecrClient := aws.NewClient()
				cfg := awssdk.NewConfig()
				cfg.EndpointResolverWithOptions = awssdk.EndpointResolverWithOptionsFunc(
					func(service, region string, options ...interface{}) (awssdk.Endpoint, error) {
						return awssdk.Endpoint{URL: serverURL}, nil
					})
				cfg.Credentials = credentials.NewStaticCredentialsProvider("x", "y", "z")
				ecrClient.WithConfig(cfg)

				mgr.WithECRClient(ecrClient)

				*image = "012345678901.dkr.ecr.us-east-1.amazonaws.com/foo:v1"
			},
		},
		{
			name:         "gcr",
			responseBody: `{"access_token": "some-token","expires_in": 10, "token_type": "foo"}`,
			providerOpts: ProviderOptions{GcpAutoLogin: true},
			beforeFunc: func(serverURL string, mgr *Manager, image *string) {
				// Create GCR client and configure the manager.
				gcrClient := gcp.NewClient().WithTokenURL(serverURL)
				mgr.WithGCRClient(gcrClient)

				*image = "gcr.io/foo/bar:v1"
			},
		},
		{
			name:         "acr",
			responseBody: `{"refresh_token": "bbbbb"}`,
			providerOpts: ProviderOptions{AzureAutoLogin: true},
			beforeFunc: func(serverURL string, mgr *Manager, image *string) {
				acrClient := azure.NewClient().WithTokenCredential(&azure.FakeTokenCredential{Token: "foo"}).WithScheme("http")
				mgr.WithACRClient(acrClient)

				*image = "foo.azurecr.io/bar:v1"
			},
			// NOTE: This fails because the azure exchanger uses the image host
			// to exchange token which can't be modified here without
			// interfering image name based categorization of the login
			// provider, that's actually being tested here. This is tested in
			// detail in the azure package.
			wantErr: true,
		},
		{
			name:         "generic",
			providerOpts: ProviderOptions{},
			beforeFunc: func(serverURL string, mgr *Manager, image *string) {
				*image = "foo/bar:v1"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Create test server.
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write([]byte(tt.responseBody))
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			mgr := NewManager()
			var image string

			if tt.beforeFunc != nil {
				tt.beforeFunc(srv.URL, mgr, &image)
			}

			ref, err := name.ParseReference(image)
			g.Expect(err).ToNot(HaveOccurred())

			_, err = mgr.Login(context.TODO(), image, ref, tt.providerOpts)
			g.Expect(err != nil).To(Equal(tt.wantErr))
		})
	}
}
