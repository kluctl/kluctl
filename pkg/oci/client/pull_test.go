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

package client

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/fluxcd/pkg/oci"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	. "github.com/onsi/gomega"
)

func Test_PullAnyTarball(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	c := NewClient(DefaultOptions())
	testDir := "testdata/artifact"

	tag := "latest"
	repo := "test-no-annotations" + randStringRunes(5)

	dst := fmt.Sprintf("%s/%s:%s", dockerReg, repo, tag)

	artifact := filepath.Join(t.TempDir(), "artifact.tgz")
	g.Expect(c.Build(artifact, testDir, nil)).To(Succeed())

	img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, oci.CanonicalConfigMediaType)

	layer, err := tarball.LayerFromFile(artifact, tarball.WithMediaType("application/vnd.acme.some.content.layer.v1.tar+gzip"))
	g.Expect(err).ToNot(HaveOccurred())

	img, err = mutate.Append(img, mutate.Addendum{Layer: layer})
	g.Expect(err).ToNot(HaveOccurred())

	g.Expect(crane.Push(img, dst, c.optionsWithContext(ctx)...)).ToNot(HaveOccurred())

	extractTo := filepath.Join(t.TempDir(), "artifact")
	m, err := c.Pull(ctx, dst, extractTo)
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(m).ToNot(BeNil())
	g.Expect(m.Annotations).To(BeEmpty())
	g.Expect(m.Created).To(BeEmpty())
	g.Expect(m.Revision).To(BeEmpty())
	g.Expect(m.Source).To(BeEmpty())
	g.Expect(m.URL).To(Equal(dst))
	g.Expect(m.Digest).ToNot(BeEmpty())
	g.Expect(extractTo).To(BeADirectory())

	for _, entry := range []string{
		"deploy",
		"deploy/repo.yaml",
		"deployment.yaml",
		"ignore-dir",
		"ignore-dir/deployment.yaml",
		"ignore.txt",
		"somedir",
		"somedir/repo.yaml",
		"somedir/git/repo.yaml",
	} {
		g.Expect(extractTo + "/" + entry).To(Or(BeAnExistingFile(), BeADirectory()))
	}
}
