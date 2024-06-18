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

package keyservice

import (
	"context"
	"fmt"

	"github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/azkv"
	"github.com/getsops/sops/v3/gcpkms"
	"github.com/getsops/sops/v3/hcvault"
	"github.com/getsops/sops/v3/keys"
	"github.com/getsops/sops/v3/keyservice"
	awskms "github.com/getsops/sops/v3/kms"
	"github.com/getsops/sops/v3/pgp"
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
	case *awskms.MasterKey:
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
