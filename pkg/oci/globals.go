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

package oci

import "github.com/google/go-containerregistry/pkg/v1/types"

var (
	// CanonicalConfigMediaType is the OCI media type for the config layer.
	CanonicalConfigMediaType types.MediaType = "application/vnd.cncf.flux.config.v1+json"

	// CanonicalContentMediaType is the OCI media type for the content layer.
	CanonicalContentMediaType types.MediaType = "application/vnd.cncf.flux.content.v1.tar+gzip"

	// UserAgent string used for OCI calls.
	UserAgent = "flux/v2"
)
