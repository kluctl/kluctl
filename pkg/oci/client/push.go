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
	"github.com/go-git/go-git/v5/plumbing/format/gitignore"
	"os"
	"path/filepath"
	"time"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/kluctl/kluctl/v2/pkg/oci"
)

// LayerType is an enumeration of the supported layer types
// when pushing an image.
type LayerType string

const (
	// LayerTypeTarball produces a layer that contains a gzipped archive
	LayerTypeTarball LayerType = "tarball"
	// LayerTypeStatic produces a layer that contains the contents of a
	// file without any compression.
	LayerTypeStatic LayerType = "static"
)

// PushOptions are options for configuring the Push operation.
type PushOptions struct {
	layerType LayerType
	layerOpts layerOptions
	meta      Metadata
}

// layerOptions are options for configuring a layer.
type layerOptions struct {
	mediaTypeExt string

	ignorePatterns []gitignore.Pattern
}

// PushOption is a function for configuring PushOptions.
type PushOption func(o *PushOptions)

// WithPushLayerType set the layer type that will be used when creating
// the image layer.
func WithPushLayerType(l LayerType) PushOption {
	return func(o *PushOptions) {
		o.layerType = l
	}
}

// WithPushMediaTypeExt configures the media type extension for the image layer.
// This is only used when the layer type is `LayerTypeStatic`.
// The final media type will be prefixed with `application/vnd.cncf.flux.content.v1`
func WithPushMediaTypeExt(extension string) PushOption {
	return func(o *PushOptions) {
		o.layerOpts.mediaTypeExt = extension
	}
}

// WithPushIgnoreFileName specified the ignore file name
func WithPushIgnorePatterns(patterns []gitignore.Pattern) PushOption {
	return func(o *PushOptions) {
		o.layerOpts.ignorePatterns = patterns
	}
}

// WithPushMetadata configures Metadata that will be used for image annotations.
func WithPushMetadata(meta Metadata) PushOption {
	return func(o *PushOptions) {
		o.meta = meta
	}
}

// Push creates an artifact from the given path, uploads the artifact
// to the given OCI repository and returns the digest.
func (c *Client) Push(ctx context.Context, url, sourcePath string, opts ...PushOption) (string, error) {
	o := &PushOptions{
		layerType: LayerTypeTarball,
	}

	for _, opt := range opts {
		opt(o)
	}
	ref, err := name.ParseReference(url)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	layer, err := createLayer(sourcePath, o.layerType, o.layerOpts)
	if err != nil {
		return "", fmt.Errorf("error creating layer: %w", err)
	}

	if o.meta.Created == "" {
		ct := time.Now().UTC()
		o.meta.Created = ct.Format(time.RFC3339)
	}

	img := mutate.MediaType(empty.Image, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, oci.CanonicalConfigMediaType)
	img = mutate.Annotations(img, o.meta.ToAnnotations()).(gcrv1.Image)

	img, err = mutate.Append(img, mutate.Addendum{Layer: layer})
	if err != nil {
		return "", fmt.Errorf("appeding content to artifact failed: %w", err)
	}

	if err := crane.Push(img, url, c.optionsWithContext(ctx)...); err != nil {
		return "", fmt.Errorf("pushing artifact failed: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("parsing artifact digest failed: %w", err)
	}

	return ref.Context().Digest(digest.String()).String(), err
}

// createLayer creates a layer depending on the layerType.
func createLayer(path string, layerType LayerType, opts layerOptions) (gcrv1.Layer, error) {
	switch layerType {
	case LayerTypeTarball:
		var ociMediaType = oci.CanonicalContentMediaType
		var tmpDir string
		tmpDir, err := os.MkdirTemp("", "oci")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tmpDir)
		tmpFile := filepath.Join(tmpDir, "artifact.tgz")
		if err := buildWithIgnorePatterns(tmpFile, path, opts.ignorePatterns); err != nil {
			return nil, err
		}
		return tarball.LayerFromFile(tmpFile, tarball.WithMediaType(ociMediaType), tarball.WithCompressedCaching)
	case LayerTypeStatic:
		var ociMediaType = getLayerMediaType(opts.mediaTypeExt)
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("error reading file for static layer: %w", err)
		}
		return static.NewLayer(content, ociMediaType), nil
	default:
		return nil, fmt.Errorf("unsupported layer type: '%s'", layerType)
	}
}

func getLayerMediaType(extension string) types.MediaType {
	if extension == "" {
		return oci.CanonicalMediaTypePrefix
	}
	return types.MediaType(fmt.Sprintf("%s.%s", oci.CanonicalMediaTypePrefix, extension))
}
