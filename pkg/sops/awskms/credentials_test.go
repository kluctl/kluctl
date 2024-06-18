/*
Copyright 2023 The Flux authors

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

package awskms

import (
	"context"
	"testing"

	. "github.com/onsi/gomega"
)

func TestLoadStaticCredentialsFromYAML(t *testing.T) {
	g := NewWithT(t)
	credsYaml := []byte(`
aws_access_key_id: test-id
aws_secret_access_key: test-secret
aws_session_token: test-token
`)
	credsProvider, err := LoadStaticCredentialsFromYAML(credsYaml)
	g.Expect(err).ToNot(HaveOccurred())

	creds, err := credsProvider.Retrieve(context.TODO())
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(creds.AccessKeyID).To(Equal("test-id"))
	g.Expect(creds.SecretAccessKey).To(Equal("test-secret"))
	g.Expect(creds.SessionToken).To(Equal("test-token"))
}
