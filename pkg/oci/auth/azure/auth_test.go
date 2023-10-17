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

package azure

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	. "github.com/onsi/gomega"
)

func TestGetAzureLoginAuth(t *testing.T) {
	tests := []struct {
		name            string
		tokenCredential azcore.TokenCredential
		responseBody    string
		statusCode      int
		wantErr         bool
		wantAuthConfig  authn.AuthConfig
	}{
		{
			name:            "success",
			tokenCredential: &FakeTokenCredential{Token: "foo"},
			responseBody:    `{"refresh_token": "bbbbb"}`,
			statusCode:      http.StatusOK,
			wantAuthConfig: authn.AuthConfig{
				Username: "00000000-0000-0000-0000-000000000000",
				Password: "bbbbb",
			},
		},
		{
			name:            "fail to get access token",
			tokenCredential: &FakeTokenCredential{Err: errors.New("no access token")},
			wantErr:         true,
		},
		{
			name:            "error from exchange service",
			tokenCredential: &FakeTokenCredential{Token: "foo"},
			responseBody:    `[{"code": "111","message": "error message 1"}]`,
			statusCode:      http.StatusInternalServerError,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Run a test server.
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			// Configure new client with test token credential.
			c := NewClient().
				WithTokenCredential(tt.tokenCredential).
				WithScheme("http")

			auth, err := c.getLoginAuth(context.TODO(), srv.URL)
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if tt.statusCode == http.StatusOK {
				g.Expect(auth).To(Equal(tt.wantAuthConfig))
			}
		})
	}
}

func TestValidHost(t *testing.T) {
	tests := []struct {
		host   string
		result bool
	}{
		{"foo.azurecr.io", true},
		{"foo.azurecr.cn", true},
		{"foo.azurecr.de", true},
		{"foo.azurecr.us", true},
		{"gcr.io", false},
		{"docker.io", false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(ValidHost(tt.host)).To(Equal(tt.result))
		})
	}
}

func TestGetCloudConfiguration(t *testing.T) {
	tests := []struct {
		host   string
		result cloud.Configuration
	}{
		{"foo.azurecr.io", cloud.AzurePublic},
		{"foo.azurecr.cn", cloud.AzureChina},
		{"foo.azurecr.de", cloud.AzurePublic},
		{"foo.azurecr.us", cloud.AzureGovernment},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(getCloudConfiguration(tt.host)).To(Equal(tt.result))
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name       string
		autoLogin  bool
		statusCode int
		testOIDC   bool
		wantErr    bool
	}{
		{
			name:       "no auto login",
			autoLogin:  false,
			statusCode: http.StatusOK,
			wantErr:    true,
		},
		{
			name:       "with auto login",
			autoLogin:  true,
			testOIDC:   true,
			statusCode: http.StatusOK,
		},
		{
			name:       "login failure",
			autoLogin:  true,
			statusCode: http.StatusInternalServerError,
			testOIDC:   true,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			// Run a test server.
			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"refresh_token": "bbbbb"}`))
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			// Construct an image repo name against the test server.
			u, err := url.Parse(srv.URL)
			g.Expect(err).ToNot(HaveOccurred())
			image := path.Join(u.Host, "foo/bar:v1")
			ref, err := name.ParseReference(image)
			g.Expect(err).ToNot(HaveOccurred())

			ac := NewClient().
				WithTokenCredential(&FakeTokenCredential{Token: "foo"}).
				WithScheme("http")

			_, err = ac.Login(context.TODO(), tt.autoLogin, u.Host, ref)
			g.Expect(err != nil).To(Equal(tt.wantErr))

			if tt.testOIDC {
				_, err = ac.OIDCLogin(context.TODO(), srv.URL)
				g.Expect(err != nil).To(Equal(tt.wantErr))
			}
		})
	}
}
