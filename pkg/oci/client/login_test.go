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

package client

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/google/go-containerregistry/pkg/crane"
	. "github.com/onsi/gomega"
)

type mockTransport struct {
	request  *http.Request
	response *http.Response
	err      error
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	m.request = req.Clone(context.TODO())
	return m.response, m.err
}

func Test_Login(t *testing.T) {
	tests := []struct {
		name         string
		creds        string
		expectedAuth string
	}{
		{
			name:         "credentials with username and password",
			creds:        "username:password",
			expectedAuth: "Basic dXNlcm5hbWU6cGFzc3dvcmQ=",
		},
		{
			name:         "credentials like a pat-token",
			creds:        "pat-token",
			expectedAuth: "Bearer pat-token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			c := NewClient(DefaultOptions())
			ctx := context.Background()
			err := c.LoginWithCredentials(tt.creds)
			g.Expect(err).ToNot(HaveOccurred())

			transportFunc := mockTransport{
				response: &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(`{}`)),
				},
			}

			c.options = append(c.options, crane.WithTransport(&transportFunc))

			err = crane.Delete(fmt.Sprintf("%s/%s:%s", dockerReg, "test", "test"), c.optionsWithContext(ctx)...)
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(transportFunc.request).ToNot(BeNil())
			g.Expect(transportFunc.request.Header.Get("Authorization")).To(Equal(tt.expectedAuth))
		})
	}
}
