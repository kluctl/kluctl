// Copyright (C) 2022 The Flux authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package keyservice

import (
	"go.mozilla.org/sops/v3/age"
	"go.mozilla.org/sops/v3/keys"
	"go.mozilla.org/sops/v3/pgp"
)

// IsOfflineMethod returns true for offline decrypt methods or false otherwise
func IsOfflineMethod(mk keys.MasterKey) bool {
	switch mk.(type) {
	case *pgp.MasterKey, *age.MasterKey:
		return true
	default:
		return false
	}
}
