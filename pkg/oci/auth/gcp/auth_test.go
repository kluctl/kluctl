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

package gcp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	. "github.com/onsi/gomega"
)

const testValidGCRImage = "gcr.io/foo/bar:v1"

func TestGetLoginAuth(t *testing.T) {
	tests := []struct {
		name           string
		responseBody   string
		statusCode     int
		wantErr        bool
		wantAuthConfig authn.AuthConfig
	}{
		{
			name: "success",
			responseBody: `{
	"access_token": "some-token",
	"expires_in": 10,
	"token_type": "foo"
}`,
			statusCode: http.StatusOK,
			wantAuthConfig: authn.AuthConfig{
				Username: "oauth2accesstoken",
				Password: "some-token",
			},
		},
		{
			name:       "fail",
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
		},
		{
			name:         "invalid response",
			responseBody: "foo",
			statusCode:   http.StatusOK,
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(tt.responseBody))
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			gc := NewClient().WithTokenURL(srv.URL)
			a, err := gc.getLoginAuth(context.TODO())
			g.Expect(err != nil).To(Equal(tt.wantErr))
			if tt.statusCode == http.StatusOK {
				g.Expect(a).To(Equal(tt.wantAuthConfig))
			}
		})
	}
}

func TestValidHost(t *testing.T) {
	tests := []struct {
		host   string
		result bool
	}{
		{"gcr.io", true},
		{"foo.gcr.io", true},
		{"foo-docker.pkg.dev", true},
		{"docker.io", false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			g := NewWithT(t)
			g.Expect(ValidHost(tt.host)).To(Equal(tt.result))
		})
	}
}

func TestLogin(t *testing.T) {
	tests := []struct {
		name       string
		autoLogin  bool
		image      string
		statusCode int
		testOIDC   bool
		wantErr    bool
	}{
		{
			name:       "no auto login",
			autoLogin:  false,
			image:      testValidGCRImage,
			statusCode: http.StatusOK,
			wantErr:    true,
		},
		{
			name:       "with auto login",
			autoLogin:  true,
			image:      testValidGCRImage,
			testOIDC:   true,
			statusCode: http.StatusOK,
		},
		{
			name:       "login failure",
			autoLogin:  true,
			image:      testValidGCRImage,
			statusCode: http.StatusInternalServerError,
			testOIDC:   true,
			wantErr:    true,
		},
		{
			name:       "non GCR image",
			autoLogin:  true,
			image:      "foo/bar:v1",
			statusCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)

			handler := func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				w.Write([]byte(`{"access_token": "some-token","expires_in": 10, "token_type": "foo"}`))
			}
			srv := httptest.NewServer(http.HandlerFunc(handler))
			t.Cleanup(func() {
				srv.Close()
			})

			ref, err := name.ParseReference(tt.image)
			g.Expect(err).ToNot(HaveOccurred())

			gc := NewClient().WithTokenURL(srv.URL)

			_, err = gc.Login(context.TODO(), tt.autoLogin, tt.image, ref)
			g.Expect(err != nil).To(Equal(tt.wantErr))

			if tt.testOIDC {
				_, err = gc.OIDCLogin(context.TODO())
				g.Expect(err != nil).To(Equal(tt.wantErr))
			}
		})
	}
}
