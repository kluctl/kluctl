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
	"os"
	"path/filepath"
	"testing"

	_ "github.com/google/go-containerregistry/pkg/crane"
	. "github.com/onsi/gomega"
)

func TestClient_Diff(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	c := NewClient(DefaultOptions())
	tag := "v0.0.1"
	repo := "test-push" + randStringRunes(5)

	url := fmt.Sprintf("%s/%s:%s", dockerReg, repo, tag)
	metadata := Metadata{
		Source:   "github.com/fluxcd/flux2",
		Revision: "rev",
	}

	testDir := "testdata/artifact"
	_, err := c.Push(ctx, url, testDir, WithPushMetadata(metadata))
	g.Expect(err).ToNot(HaveOccurred())

	err = c.Diff(ctx, url, testDir, nil)
	g.Expect(err).ToNot(HaveOccurred())

	tmpBuildDir, err := os.MkdirTemp("", "oci")
	g.Expect(err).ToNot(HaveOccurred())
	defer os.RemoveAll(tmpBuildDir)

	g.Expect(os.WriteFile(filepath.Join(tmpBuildDir, "test.txt"), []byte("test"), 0o600)).ToNot(HaveOccurred())

	newTag := "v0.0.2"
	url = fmt.Sprintf("%s/%s:%s", dockerReg, repo, newTag)

	_, err = c.Push(ctx, url, tmpBuildDir, WithPushMetadata(metadata))
	g.Expect(err).ToNot(HaveOccurred())

	err = c.Diff(ctx, url, testDir, nil)
	g.Expect(err).To(HaveOccurred())
	g.Expect(err).To(MatchError("the remote artifact contents differs from the local one"))
}
