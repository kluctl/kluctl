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

package oci

// Provider is used to categorize the OCI registry providers.
type Provider int

// Registry providers.
const (
	// ProviderGeneric is used to categorize registry provider that we don't
	// support.
	ProviderGeneric Provider = iota
	ProviderAWS
	ProviderGCP
	ProviderAzure
)

// Registry TLS transport config.
const (
	ClientCert = "certFile"
	ClientKey  = "keyFile"
	CACert     = "caFile"
)

const (
	// SourceAnnotation is the OpenContainers annotation for specifying
	// the upstream source of an OCI artifact.
	SourceAnnotation = "org.opencontainers.image.source"

	// RevisionAnnotation is the OpenContainers annotation for specifying
	// the upstream source revision of an OCI artifact.
	RevisionAnnotation = "org.opencontainers.image.revision"

	// CreatedAnnotation is the OpenContainers annotation for specifying
	// the date and time on which the OCI artifact was built (RFC 3339).
	CreatedAnnotation = "org.opencontainers.image.created"

	// OCIRepositoryPrefix is the prefix used for OCIRepository URLs.
	OCIRepositoryPrefix = "oci://"
)
