/*
Copyright 2020 The Flux authors

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

package version

import (
	"testing"
)

func TestParseVersion(t *testing.T) {
	tests := []struct {
		version string
		err     bool
	}{
		{"v1.2.3", false},
		{"v1.0", true},
		{"v1", true},
		{"v1.2.beta", true},
		{"v1.2-5", true},
		{"v1.2-beta.5", true},
		{"\nv1.2", true},
		{"v1.2.0-x.Y.0+metadata", false},
		{"v1.2.0-x.Y.0+metadata-width-hypen", false},
		{"v1.2.3-rc1-with-hypen", false},
		{"v1.2.3.4", true},
	}

	for _, tc := range tests {
		_, err := ParseVersion(tc.version)
		if tc.err && err == nil {
			t.Fatalf("expected error for version: %s", tc.version)
		} else if !tc.err && err != nil {
			t.Fatalf("error for version %s: %s", tc.version, err)
		}
	}
}
