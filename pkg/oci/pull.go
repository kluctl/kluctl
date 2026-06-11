package oci

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/fluxcd/pkg/oci"
	"github.com/fluxcd/pkg/tar"
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/cache"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

var (
	// gzipMagicHeader are bytes found at the start of gzip files
	// https://github.com/google/go-containerregistry/blob/a54d64203cffcbf94146e04069aae4a97f228ee2/internal/gzip/zip.go#L28
	gzipMagicHeader = []byte{'\x1f', byte(int('\x8b'))}
)

func PullCached(ctx context.Context, craneOptions []crane.Option, c cache.Cache, url string, outPath string) (*oci.Metadata, error) {
	var craneOptions2 []crane.Option
	craneOptions2 = append(craneOptions2, craneOptions...)
	craneOptions2 = append(craneOptions2, crane.WithContext(ctx))

	opts := crane.GetOptions(craneOptions2...)

	ref, err := name.ParseReference(url, opts.Name...)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	img, err := remote.Image(ref, opts.Remote...)
	if err != nil {
		return nil, err
	}

	c = &dummyCacheDiffIdWrapper{c}
	img = cache.Image(img, c)

	digest, err := img.Digest()
	if err != nil {
		return nil, fmt.Errorf("parsing digest failed: %w", err)
	}

	manifest, err := img.Manifest()
	if err != nil {
		return nil, fmt.Errorf("parsing manifest failed: %w", err)
	}

	meta := oci.MetadataFromAnnotations(manifest.Annotations)
	meta.URL = url
	meta.Digest = ref.Context().Digest(digest.String()).String()

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to list layers: %w", err)
	}

	if len(layers) < 1 {
		return nil, fmt.Errorf("no layers found in artifact")
	}

	if len(layers) < 1 {
		return nil, fmt.Errorf("index '%d' out of bound for '%d' layers in artifact", 0, len(layers))
	}

	err = extractLayer(layers[0], outPath, "")
	if err != nil {
		return nil, err
	}
	return meta, nil
}

type dummyDiffIdWrapper struct {
	gcrv1.Layer
}

type dummyCacheDiffIdWrapper struct {
	cache.Cache
}

func (d *dummyCacheDiffIdWrapper) Put(l gcrv1.Layer) (gcrv1.Layer, error) {
	l = &dummyDiffIdWrapper{Layer: l}
	return d.Cache.Put(l)
}

func (d *dummyDiffIdWrapper) DiffID() (gcrv1.Hash, error) {
	return d.Digest()
}

// extractLayer extracts the Layer to the path
func extractLayer(layer gcrv1.Layer, path string, layerType oci.LayerType) error {
	// cached images required DiffID to work, but this only works for images with a rootfs (which we don't have)
	layer = &dummyDiffIdWrapper{layer}

	var blob io.Reader
	blob, err := layer.Compressed()
	if err != nil {
		return fmt.Errorf("extracting layer failed: %w", err)
	}

	actualLayerType := layerType
	if actualLayerType == "" {
		bufReader := bufio.NewReader(blob)
		if ok, _ := isGzipBlob(bufReader); ok {
			actualLayerType = oci.LayerTypeTarball
		} else {
			actualLayerType = oci.LayerTypeStatic
		}
		// the bufio.Reader has read the bytes from the io.Reader
		// and should be used instead
		blob = bufReader
	}

	return extractLayerType(path, blob, actualLayerType)
}

// extractLayerType extracts the contents of a io.Reader to the given path.
// If the LayerType is LayerTypeTarball, it will untar to a directory,
// If the LayerType is LayerTypeStatic, it will copy to a file.
func extractLayerType(path string, blob io.Reader, layerType oci.LayerType) error {
	switch layerType {
	case oci.LayerTypeTarball:
		return tar.Untar(blob, path, tar.WithMaxUntarSize(-1), tar.WithSkipSymlinks())
	case oci.LayerTypeStatic:
		f, err := os.Create(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(f, blob)
		if err != nil {
			return fmt.Errorf("error copying layer content: %s", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported layer type: '%s'", layerType)
	}
}

// isGzipBlob reads the first two bytes from a bufio.Reader and
// checks that they are equal to the expected gzip file headers.
func isGzipBlob(buf *bufio.Reader) (bool, error) {
	b, err := buf.Peek(len(gzipMagicHeader))
	if err != nil {
		if err == io.EOF {
			return false, nil
		}
		return false, err
	}
	return bytes.Equal(b, gzipMagicHeader), nil
}
