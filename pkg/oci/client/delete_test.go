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
	gcrv1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/random"
	. "github.com/onsi/gomega"
	"testing"
	"time"
)

func TestDelete(t *testing.T) {
	g := NewWithT(t)
	ctx := context.Background()
	c := NewClient(DefaultOptions())
	repo := "test-delete" + randStringRunes(5)
	tags := []string{"v0.0.1", "v0.0.2", "v0.0.3", "latest"}
	source := "github.com/fluxcd/fluxv2"
	rev := "rev"
	ct := time.Now()
	m := Metadata{
		Source:   source,
		Revision: rev,
		Created:  ct.Format(time.RFC3339),
	}

	for _, tag := range tags {
		dst := fmt.Sprintf("%s/%s:%s", dockerReg, repo, tag)
		img, err := random.Image(1024, 1)
		g.Expect(err).ToNot(HaveOccurred())
		img = mutate.Annotations(img, m.ToAnnotations()).(gcrv1.Image)
		err = crane.Push(img, dst, c.options...)
		g.Expect(err).ToNot(HaveOccurred())
	}

	tests := []struct {
		name        string
		url         string
		expectedErr bool
		checkTags   []string
	}{
		{
			name:      "delete with no tag (defaults to latest)",
			url:       fmt.Sprintf("%s/%s", dockerReg, repo),
			checkTags: []string{"latest"},
		},
		{
			name:      "delete tag",
			url:       fmt.Sprintf("%s/%s:v0.0.1", dockerReg, repo),
			checkTags: []string{"v0.0.1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := NewWithT(t)
			err := c.Delete(ctx, tt.url)
			if tt.expectedErr {
				g.Expect(err).ToNot(BeNil())
				return
			}

			g.Expect(err).To(BeNil())

			for _, tag := range tt.checkTags {
				_, err = crane.Pull(fmt.Sprintf("%s/%s:%s", dockerReg, repo, tag))
				g.Expect(err).ToNot(BeNil())
				g.Expect(err.Error()).To(ContainSubstring("manifest unknown"))
			}
		})
	}
}
