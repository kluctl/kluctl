// Copyright (C) 2022 The Flux authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package keyservice

import (
	"context"
	"fmt"
	"go.mozilla.org/sops/v3/kms"

	"go.mozilla.org/sops/v3/keys"
	"go.mozilla.org/sops/v3/keyservice"

	"go.mozilla.org/sops/v3/age"
	"go.mozilla.org/sops/v3/azkv"
	"go.mozilla.org/sops/v3/gcpkms"
	"go.mozilla.org/sops/v3/hcvault"
	"go.mozilla.org/sops/v3/pgp"
)

// KeyFromMasterKey converts a SOPS internal MasterKey to an RPC Key that can
// be serialized with Protocol Buffers.
func KeyFromMasterKey(k keys.MasterKey) keyservice.Key {
	switch mk := k.(type) {
	case *pgp.MasterKey:
		return keyservice.Key{
			KeyType: &keyservice.Key_PgpKey{
				PgpKey: &keyservice.PgpKey{
					Fingerprint: mk.Fingerprint,
				},
			},
		}
	case *hcvault.MasterKey:
		return keyservice.Key{
			KeyType: &keyservice.Key_VaultKey{
				VaultKey: &keyservice.VaultKey{
					VaultAddress: mk.VaultAddress,
					EnginePath:   mk.EnginePath,
					KeyName:      mk.KeyName,
				},
			},
		}
	case *kms.MasterKey:
		return keyservice.Key{
			KeyType: &keyservice.Key_KmsKey{
				KmsKey: &keyservice.KmsKey{
					Arn: mk.Arn,
				},
			},
		}
	case *azkv.MasterKey:
		return keyservice.Key{
			KeyType: &keyservice.Key_AzureKeyvaultKey{
				AzureKeyvaultKey: &keyservice.AzureKeyVaultKey{
					VaultUrl: mk.VaultURL,
					Name:     mk.Name,
					Version:  mk.Version,
				},
			},
		}
	case *age.MasterKey:
		return keyservice.Key{
			KeyType: &keyservice.Key_AgeKey{
				AgeKey: &keyservice.AgeKey{
					Recipient: mk.Recipient,
				},
			},
		}
	case *gcpkms.MasterKey:
		return keyservice.Key{
			KeyType: &keyservice.Key_GcpKmsKey{
				GcpKmsKey: &keyservice.GcpKmsKey{
					ResourceId: mk.ResourceID,
				},
			},
		}
	default:
		panic(fmt.Sprintf("tried to convert unknown MasterKey type %T to keyservice.Key", mk))
	}
}

type MockKeyServer struct {
	encryptReqs []*keyservice.EncryptRequest
	decryptReqs []*keyservice.DecryptRequest
}

func NewMockKeyServer() *MockKeyServer {
	return &MockKeyServer{
		encryptReqs: make([]*keyservice.EncryptRequest, 0),
		decryptReqs: make([]*keyservice.DecryptRequest, 0),
	}
}

func (ks *MockKeyServer) Encrypt(_ context.Context, req *keyservice.EncryptRequest) (*keyservice.EncryptResponse, error) {
	ks.encryptReqs = append(ks.encryptReqs, req)
	return nil, fmt.Errorf("not actually implemented")
}

func (ks *MockKeyServer) Decrypt(_ context.Context, req *keyservice.DecryptRequest) (*keyservice.DecryptResponse, error) {
	ks.decryptReqs = append(ks.decryptReqs, req)
	return nil, fmt.Errorf("not actually implemented")
}
