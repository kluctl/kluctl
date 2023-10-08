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
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/types"
	. "github.com/onsi/gomega"

	"github.com/fluxcd/pkg/oci"
)

func Test_Push_Pull(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	c := NewClient(DefaultOptions())
	testDir := "testdata/artifact"
	tag := "v0.0.1"
	source := "github.com/fluxcd/flux2"
	revision := "rev"
	repo := "test-push" + randStringRunes(5)
	ct := time.Now().UTC()
	created := ct.Format(time.RFC3339)

	url := fmt.Sprintf("%s/%s:%s", dockerReg, repo, tag)
	metadata := Metadata{
		Source:   source,
		Revision: revision,
		Created:  created,
		Annotations: map[string]string{
			"org.opencontainers.image.documentation": "https://my/readme.md",
			"org.opencontainers.image.licenses":      "Apache-2.0",
		},
	}

	// Build and push the artifact to registry
	_, err := c.Push(ctx, url, testDir, metadata, nil)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify that the artifact and its tag is present in the registry
	tags, err := crane.ListTags(fmt.Sprintf("%s/%s", dockerReg, repo))
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(len(tags)).To(BeEquivalentTo(1))
	g.Expect(tags[0]).To(BeEquivalentTo(tag))

	// Pull the artifact from registry
	image, err := crane.Pull(fmt.Sprintf("%s/%s:%s", dockerReg, repo, tag))
	g.Expect(err).ToNot(HaveOccurred())

	// Extract the manifest from the pulled artifact
	manifest, err := image.Manifest()
	g.Expect(err).ToNot(HaveOccurred())

	// Verify that annotations exist in manifest
	g.Expect(manifest.Annotations[oci.CreatedAnnotation]).To(BeEquivalentTo(created))
	g.Expect(manifest.Annotations[oci.SourceAnnotation]).To(BeEquivalentTo(source))
	g.Expect(manifest.Annotations[oci.RevisionAnnotation]).To(BeEquivalentTo(revision))

	// Verify media types
	g.Expect(manifest.MediaType).To(Equal(types.OCIManifestSchema1))
	g.Expect(manifest.Config.MediaType).To(BeEquivalentTo(oci.CanonicalConfigMediaType))
	g.Expect(len(manifest.Layers)).To(BeEquivalentTo(1))
	g.Expect(manifest.Layers[0].MediaType).To(BeEquivalentTo(oci.CanonicalContentMediaType))

	tmpDir := t.TempDir()

	// Pull the artifact from registry and extract its contents to tmp
	meta, err := c.Pull(ctx, url, tmpDir)
	g.Expect(err).ToNot(HaveOccurred())

	// Verify custom annotations
	g.Expect(meta.Annotations["org.opencontainers.image.documentation"]).To(BeEquivalentTo("https://my/readme.md"))
	g.Expect(meta.Annotations["org.opencontainers.image.licenses"]).To(BeEquivalentTo("Apache-2.0"))

	// Walk the test directory and check that all files exist in the pulled artifact
	fsErr := filepath.Walk(testDir, func(path string, info fs.FileInfo, err error) error {
		if !info.IsDir() {
			tmpPath := filepath.Join(tmpDir, strings.TrimPrefix(path, testDir))
			if _, err := os.Stat(tmpPath); err != nil && os.IsNotExist(err) {
				return fmt.Errorf("path '%s' doesn't exist in archive", path)
			}
		}

		return nil
	})
	g.Expect(fsErr).ToNot(HaveOccurred())
}
