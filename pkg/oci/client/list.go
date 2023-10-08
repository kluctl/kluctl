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
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/google/go-containerregistry/pkg/crane"
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"

	"github.com/fluxcd/pkg/version"
)

// ListOptions contains options for listing tags from an OCI repository.
type ListOptions struct {
	// SemverFilter contains semver for filtering tags.
	SemverFilter string
	// RegexFilter contains a regex that tags will be filtered by.
	RegexFilter string
	// IncludeCosignArtifacts can be used to include cosign attestation,
	// signature and SBOM tags in the list, as these are excluded by default.
	IncludeCosignArtifacts bool
}

// List fetches the tags and their manifests for a given OCI repository.
func (c *Client) List(ctx context.Context, url string, opts ListOptions) ([]Metadata, error) {
	metas := make([]Metadata, 0)
	tags, err := crane.ListTags(url, c.options...)
	if err != nil {
		return nil, fmt.Errorf("listing tags failed: %w", err)
	}

	sort.Slice(tags, func(i, j int) bool { return tags[i] > tags[j] })

	var constraint *semver.Constraints
	if opts.SemverFilter != "" {
		constraint, err = semver.NewConstraint(opts.SemverFilter)
		if err != nil {
			return nil, fmt.Errorf("semver '%s' parse error: %w", opts.SemverFilter, err)
		}
	}

	var re *regexp.Regexp
	if opts.RegexFilter != "" {
		re, err = regexp.Compile(opts.RegexFilter)
		if err != nil {
			return nil, fmt.Errorf("regex '%s' parse error: %w", opts.RegexFilter, err)
		}
	}

	for _, tag := range tags {
		// ignore cosign artifacts by default
		if !opts.IncludeCosignArtifacts && IsCosignArtifact(tag) {
			continue
		}

		if constraint != nil {
			v, err := version.ParseVersion(tag)
			// version isn't a valid semver so we can skip
			if err != nil {
				continue
			}

			if !constraint.Check(v) {
				continue
			}
		}

		if re != nil && !re.Match([]byte(tag)) {
			continue
		}

		meta := Metadata{
			URL: fmt.Sprintf("%s:%s", url, tag),
		}

		manifestJSON, err := crane.Manifest(meta.URL, c.optionsWithContext(ctx)...)
		if err != nil {
			return nil, fmt.Errorf("fetching manifest failed: %w", err)
		}

		manifest, err := gcrv1.ParseManifest(bytes.NewReader(manifestJSON))
		if err != nil {
			return nil, fmt.Errorf("parsing manifest failed: %w", err)
		}

		manifestMetadata := MetadataFromAnnotations(manifest.Annotations)
		meta.Revision = manifestMetadata.Revision
		meta.Source = manifestMetadata.Source
		meta.Created = manifestMetadata.Created

		digest, err := crane.Digest(meta.URL, c.optionsWithContext(ctx)...)
		if err != nil {
			return nil, fmt.Errorf("fetching digest failed: %w", err)
		}
		meta.Digest = digest

		metas = append(metas, meta)
	}

	return metas, nil
}

// IsCosignArtifact will return true if the tag has one of the following suffices:
// ".att", ".sbom", or ".sig". These are the suffices used by cosign to store the
// attestations, SBOMs, and signatures respectively.
func IsCosignArtifact(tag string) bool {
	for _, suffix := range []string{".att", ".sbom", ".sig"} {
		if strings.HasSuffix(tag, suffix) {
			return true
		}
	}
	return false
}
