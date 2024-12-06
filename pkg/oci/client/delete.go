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
	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
)

// Delete deletes a particular image from an OCI repository
// If the url has no tag, the latest image is deleted
func (c *Client) Delete(ctx context.Context, url string) error {
	_, err := name.ParseReference(url)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}

	return crane.Delete(url, c.optionsWithContext(ctx)...)
}
