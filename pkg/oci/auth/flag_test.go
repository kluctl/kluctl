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

// Make sure we never inject non-test flags when the auth packages are imported.
// Refer https://github.com/fluxcd/pkg/issues/645.
package auth_test

import (
	"flag"
	"strings"
	"testing"

	_ "github.com/fluxcd/pkg/oci/auth/login"
)

func TestNonTestFlagCheck(t *testing.T) {
	flagCheck := func(f *flag.Flag) {
		if !strings.HasPrefix(f.Name, "test.") {
			t.Errorf("found non-test command line flag: %q", f.Name)
		}
	}
	flag.VisitAll(flagCheck)
}
