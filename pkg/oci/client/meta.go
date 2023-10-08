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
	"github.com/fluxcd/pkg/oci"
)

// Metadata holds the upstream information about on artifact's source.
// https://github.com/opencontainers/image-spec/blob/main/annotations.md
type Metadata struct {
	Created     string            `json:"created,omitempty"`
	Source      string            `json:"source_url,omitempty"`
	Revision    string            `json:"source_revision,omitempty"`
	Digest      string            `json:"digest"`
	URL         string            `json:"url"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// ToAnnotations returns the OpenContainers annotations map.
func (m *Metadata) ToAnnotations() map[string]string {
	annotations := map[string]string{
		oci.CreatedAnnotation:  m.Created,
		oci.SourceAnnotation:   m.Source,
		oci.RevisionAnnotation: m.Revision,
	}

	for k, v := range m.Annotations {
		annotations[k] = v
	}

	return annotations
}

// MetadataFromAnnotations parses the OpenContainers annotations and returns a Metadata object.
func MetadataFromAnnotations(annotations map[string]string) *Metadata {
	return &Metadata{
		Created:     annotations[oci.CreatedAnnotation],
		Source:      annotations[oci.SourceAnnotation],
		Revision:    annotations[oci.RevisionAnnotation],
		Annotations: annotations,
	}
}
