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
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	. "github.com/onsi/gomega"

	"github.com/kluctl/kluctl/v2/pkg/oci"
)

func Test_Push_Pull(t *testing.T) {
	ctx := context.Background()
	c := NewClient(DefaultOptions())
	source := "github.com/fluxcd/flux2"
	revision := "rev"
	repo := "test-push" + randStringRunes(5)
	ct := time.Now().UTC()
	created := ct.Format(time.RFC3339)

	tests := []struct {
		name              string
		sourcePath        string
		tag               string
		ignorePaths       []string
		pushOpts          []PushOption
		pullOpts          []PullOption
		pushFn            func(url string, path string) error
		testLayerType     LayerType
		testLayerIndex    int
		expectedNumLayers int
		expectedPullErr   bool
		expectedPushErr   bool
		expectedMediaType types.MediaType
	}{
		{
			name:              "push directory (default layer type)",
			tag:               "v0.0.1",
			sourcePath:        "testdata/artifact",
			testLayerType:     LayerTypeTarball,
			expectedMediaType: oci.CanonicalContentMediaType,
		},
		{
			name:       "push directory (specify layer type)",
			tag:        "v0.0.1",
			sourcePath: "testdata/artifact",
			pushOpts: []PushOption{
				WithPushLayerType(LayerTypeTarball),
			},
			pullOpts: []PullOption{
				WithPullLayerType(LayerTypeTarball),
			},
			testLayerType:     LayerTypeTarball,
			expectedMediaType: oci.CanonicalContentMediaType,
		},
		{
			name:       "push static file",
			tag:        "v0.0.2",
			sourcePath: "testdata/artifact/deployment.yaml",
			pushOpts: []PushOption{
				WithPushLayerType(LayerTypeStatic),
				WithPushMediaTypeExt("ml"),
			},
			pullOpts: []PullOption{
				WithPullLayerType(LayerTypeStatic),
			},
			testLayerType:     LayerTypeStatic,
			expectedMediaType: getLayerMediaType("ml"),
		},
		{
			name:       "push directory as static layer (should return error)",
			sourcePath: "testdata/artifact",
			pushOpts: []PushOption{
				WithPushLayerType(LayerTypeStatic),
			},
			expectedPushErr: true,
		},
		{
			name:       "push static file without media type extension (automatic layer detection for pull)",
			tag:        "v0.0.2",
			sourcePath: "testdata/artifact/deployment.yaml",
			pushOpts: []PushOption{
				WithPushLayerType(LayerTypeStatic),
			},
			testLayerType:     LayerTypeStatic,
			expectedMediaType: oci.CanonicalMediaTypePrefix,
		},
		{
			name:       "push directory and pull archive (push with LayerTypeTarball and pull with LayerTypeStatic)",
			tag:        "v0.0.2",
			sourcePath: "testdata/artifact",
			pullOpts: []PullOption{
				WithPullLayerType(LayerTypeStatic),
			},
			testLayerType:     LayerTypeStatic,
			expectedMediaType: oci.CanonicalContentMediaType,
		},
		{
			name:              "push static artifact (and with LayerTypeTarball PullOption - should return error)",
			tag:               "static-flux",
			sourcePath:        "testdata/artifact/deployment.yaml",
			pullOpts:          []PullOption{WithPullLayerType(LayerTypeTarball)},
			pushOpts:          []PushOption{WithPushLayerType(LayerTypeStatic)},
			expectedPullErr:   true,
			expectedMediaType: oci.CanonicalMediaTypePrefix,
		},
		{
			name:       "two layers in image (specify index)",
			tag:        "two-layers",
			sourcePath: "testdata/artifact",
			pullOpts:   []PullOption{WithPullLayerType(LayerTypeTarball), WithPullLayerIndex(1)},
			pushFn: func(url string, path string) error {
				artifact := filepath.Join(t.TempDir(), "artifact.tgz")
				err := buildWithIgnorePaths(artifact, path, nil)
				if err != nil {
					return err
				}

				img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
				img = mutate.ConfigMediaType(img, oci.CanonicalConfigMediaType)

				layer1 := static.NewLayer([]byte("test-byte"), oci.CanonicalMediaTypePrefix)

				layer2, err := tarball.LayerFromFile(artifact, tarball.WithMediaType(oci.CanonicalContentMediaType))
				if err != nil {
					return err
				}

				img, err = mutate.Append(img, mutate.Addendum{Layer: layer1}, mutate.Addendum{Layer: layer2})
				if err != nil {
					return err
				}

				err = crane.Push(img, url, c.optionsWithContext(ctx)...)
				if err != nil {
					return err
				}
				return err
			},
			testLayerIndex:    1,
			testLayerType:     LayerTypeTarball,
			expectedNumLayers: 2,
			expectedMediaType: oci.CanonicalContentMediaType,
		},
		{
			name:       "specify wrong layer (should return error)",
			tag:        "not-flux",
			sourcePath: "testdata/artifact",
			pullOpts:   []PullOption{WithPullLayerType(LayerTypeTarball), WithPullLayerIndex(1)},
			pushFn: func(url string, path string) error {
				artifact := filepath.Join(t.TempDir(), "artifact.tgz")
				err := buildWithIgnorePaths(artifact, path, nil)
				if err != nil {
					return err
				}

				img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
				img = mutate.ConfigMediaType(img, oci.CanonicalConfigMediaType)

				layer1, err := tarball.LayerFromFile(artifact, tarball.WithMediaType(oci.CanonicalContentMediaType))
				if err != nil {
					return err
				}

				img, err = mutate.Append(img, mutate.Addendum{Layer: layer1})
				if err != nil {
					return err
				}

				dst := fmt.Sprintf("%s/%s:%s", dockerReg, repo, "not-flux")
				err = crane.Push(img, dst, c.optionsWithContext(ctx)...)
				if err != nil {
					return err
				}
				return err
			},
			expectedMediaType: oci.CanonicalContentMediaType,
			expectedPullErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			url := fmt.Sprintf("%s/%s:%s", dockerReg, repo, tt.tag)
			metadata := Metadata{
				Source:   source,
				Revision: revision,
				Created:  created,
				Annotations: map[string]string{
					"org.opencontainers.image.documentation": "https://my/readme.md",
					"org.opencontainers.image.licenses":      "Apache-2.0",
				},
			}
			opts := append(tt.pushOpts, WithPushMetadata(metadata))

			// Build and push the artifact to registry
			var err error
			if tt.pushFn == nil {
				_, err = c.Push(ctx, url, tt.sourcePath, opts...)
			} else {
				err = tt.pushFn(url, tt.sourcePath)
			}
			if tt.expectedPushErr {
				g.Expect(err).To(HaveOccurred())
				return
			}

			g.Expect(err).To(Not(HaveOccurred()))
			// Verify that the artifact and its tag is present in the registry
			tags, err := crane.ListTags(fmt.Sprintf("%s/%s", dockerReg, repo))
			g.Expect(err).ToNot(HaveOccurred())
			g.Expect(tags).To(ContainElement(tt.tag))

			// Pull the artifact from registry
			image, err := crane.Pull(fmt.Sprintf("%s/%s:%s", dockerReg, repo, tt.tag))
			g.Expect(err).ToNot(HaveOccurred())

			// Extract the manifest from the pulled artifact
			manifest, err := image.Manifest()
			g.Expect(err).ToNot(HaveOccurred())

			// Verify that annotations exist in manifest
			if len(manifest.Annotations) != 0 {
				g.Expect(manifest.Annotations[oci.CreatedAnnotation]).To(BeEquivalentTo(created))
				g.Expect(manifest.Annotations[oci.SourceAnnotation]).To(BeEquivalentTo(source))
				g.Expect(manifest.Annotations[oci.RevisionAnnotation]).To(BeEquivalentTo(revision))
			}

			// Verify media types
			g.Expect(manifest.MediaType).To(Equal(types.OCIManifestSchema1))
			g.Expect(manifest.Config.MediaType).To(BeEquivalentTo(oci.CanonicalConfigMediaType))

			numLayers := 1
			layerIdx := 0
			if tt.expectedNumLayers > 0 {
				numLayers = tt.expectedNumLayers
				layerIdx = tt.testLayerIndex
			}
			g.Expect(len(manifest.Layers)).To(BeEquivalentTo(numLayers))
			g.Expect(manifest.Layers[layerIdx].MediaType).To(BeEquivalentTo(tt.expectedMediaType))

			// Verify custom annotations
			meta := MetadataFromAnnotations(manifest.Annotations)
			if len(meta.Annotations) > 0 {
				g.Expect(meta.Annotations["org.opencontainers.image.documentation"]).To(BeEquivalentTo("https://my/readme.md"))
				g.Expect(meta.Annotations["org.opencontainers.image.licenses"]).To(BeEquivalentTo("Apache-2.0"))
			}

			// Pull the artifact from registry and extract its contents to tmp
			tmpPath := filepath.Join(t.TempDir(), "artifact")
			_, err = c.Pull(ctx, url, tmpPath, tt.pullOpts...)
			if tt.expectedPullErr {
				g.Expect(err).To(HaveOccurred())
				return
			}
			g.Expect(err).ToNot(HaveOccurred())

			switch tt.testLayerType {
			case LayerTypeTarball:
				// Walk the test directory and check that all files exist in the pulled artifact
				fsErr := filepath.Walk(tt.sourcePath, func(path string, info fs.FileInfo, err error) error {
					if !info.IsDir() {
						tmpPath := filepath.Join(tmpPath, strings.TrimPrefix(path, tt.sourcePath))
						if _, err := os.Stat(tmpPath); err != nil && os.IsNotExist(err) {
							return fmt.Errorf("path '%s' doesn't exist in archive", path)
						}
					}

					return nil
				})
				g.Expect(fsErr).ToNot(HaveOccurred())
			case LayerTypeStatic:
				got, err := os.ReadFile(tmpPath)
				g.Expect(err).ToNot(HaveOccurred())

				fileInfo, err := os.Stat(tt.sourcePath)
				// if a directory was pushed, then the created file should be a gzipped archive
				if fileInfo.IsDir() {
					bufReader := bufio.NewReader(bytes.NewReader(got))
					g.Expect(isGzipBlob(bufReader)).To(BeTrue())
					return
				}

				expected, err := os.ReadFile(tt.sourcePath)
				g.Expect(err).ToNot(HaveOccurred())

				g.Expect(expected).To(Equal(got))
			default:
				t.Errorf("no layer type specified for test")
			}
		})
	}
}

func Test_getLayerMediaType(t *testing.T) {
	tests := []struct {
		name              string
		ext               string
		expectedMediaType types.MediaType
	}{
		{
			name:              "default oci media type",
			expectedMediaType: oci.CanonicalMediaTypePrefix,
		},
		{
			name:              "oci media type with extension",
			ext:               "test",
			expectedMediaType: types.MediaType(fmt.Sprintf("%s.test", oci.CanonicalMediaTypePrefix)),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			got := getLayerMediaType(tt.ext)
			g.Expect(got).To(BeEquivalentTo(tt.expectedMediaType))
		})
	}
}
