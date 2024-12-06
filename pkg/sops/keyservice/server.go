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
	"fmt"

	"github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/azkv"
	"github.com/getsops/sops/v3/gcpkms"
	"github.com/getsops/sops/v3/hcvault"
	"github.com/getsops/sops/v3/keyservice"
	awskms "github.com/getsops/sops/v3/kms"
	"github.com/getsops/sops/v3/logging"
	"github.com/getsops/sops/v3/pgp"
	"golang.org/x/net/context"

	intazkv "github.com/kluctl/kluctl/v2/pkg/sops/azkv"
)

// Server is a key service server that uses SOPS MasterKeys to fulfill
// requests. It intercepts Encrypt and Decrypt requests made for key types
// that need to run in a contained environment, instead of the default
// implementation which heavily utilizes environment variables or the runtime
// environment. Any request not handled by the Server is forwarded to the
// embedded default server.
type Server struct {
	// gnuPGHome is the GnuPG home directory used for the Encrypt and Decrypt
	// operations for PGP key types.
	// When empty, the requests will be handled using the systems' runtime
	// keyring.
	gnuPGHome pgp.GnuPGHome

	// ageIdentities are the parsed age identities used for Decrypt
	// operations for age key types.
	ageIdentities age.ParsedIdentities

	// vaultToken is the token used for Encrypt and Decrypt operations of
	// Hashicorp Vault requests.
	// When empty, the request will be handled by defaultServer.
	vaultToken hcvault.Token

	// azureToken is the credential token used for Encrypt and Decrypt
	// operations of Azure Key Vault requests.
	// When nil, the request will be handled by defaultServer.
	azureToken *azkv.TokenCredential

	// awsCredsProvider is the Credentials object used for Encrypt and Decrypt
	// operations of AWS KMS requests.
	// When nil, the request will be handled by defaultServer.
	awsCredsProvider *awskms.CredentialsProvider

	// gcpCredsJSON is the JSON credentials used for Decrypt and Encrypt
	// operations of GCP KMS requests. When nil, a default client with
	// environmental runtime settings will be used.
	gcpCredsJSON gcpkms.CredentialJSON

	// defaultServer is the fallback server, used to handle any request that
	// is not eligible to be handled by this Server.
	defaultServer keyservice.KeyServiceServer
}

// NewServer constructs a new Server, configuring it with the provided options
// before returning the result.
// When WithDefaultServer() is not provided as an option, the SOPS server
// implementation is configured as default.
func NewServer(options ...ServerOption) keyservice.KeyServiceServer {
	s := &Server{}
	for _, opt := range options {
		opt.ApplyToServer(s)
	}

	if s.defaultServer == nil {
		s.defaultServer = &keyservice.Server{
			Prompt: false,
		}
	}

	// Effectively disable any logging from the key services.
	// 0 equals to panic level, which should never be reached.
	logging.SetLevel(0)

	return s
}

// Encrypt takes an encrypt request and encrypts the provided plaintext with
// the provided key, returning the encrypted result.
func (ks Server) Encrypt(ctx context.Context, req *keyservice.EncryptRequest) (*keyservice.EncryptResponse, error) {
	key := req.Key
	switch k := key.KeyType.(type) {
	case *keyservice.Key_PgpKey:
		ciphertext, err := ks.encryptWithPgp(k.PgpKey, req.Plaintext)
		if err != nil {
			return nil, err
		}
		return &keyservice.EncryptResponse{
			Ciphertext: ciphertext,
		}, nil
	case *keyservice.Key_AgeKey:
		ciphertext, err := ks.encryptWithAge(k.AgeKey, req.Plaintext)
		if err != nil {
			return nil, err
		}
		return &keyservice.EncryptResponse{
			Ciphertext: ciphertext,
		}, nil
	case *keyservice.Key_VaultKey:
		if ks.vaultToken != "" {
			ciphertext, err := ks.encryptWithHCVault(k.VaultKey, req.Plaintext)
			if err != nil {
				return nil, err
			}
			return &keyservice.EncryptResponse{
				Ciphertext: ciphertext,
			}, nil
		}
	case *keyservice.Key_KmsKey:
		cipherText, err := ks.encryptWithAWSKMS(k.KmsKey, req.Plaintext)
		if err != nil {
			return nil, err
		}
		return &keyservice.EncryptResponse{
			Ciphertext: cipherText,
		}, nil
	case *keyservice.Key_AzureKeyvaultKey:
		ciphertext, err := ks.encryptWithAzureKeyVault(k.AzureKeyvaultKey, req.Plaintext)
		if err != nil {
			return nil, err
		}
		return &keyservice.EncryptResponse{
			Ciphertext: ciphertext,
		}, nil
	case *keyservice.Key_GcpKmsKey:
		ciphertext, err := ks.encryptWithGCPKMS(k.GcpKmsKey, req.Plaintext)
		if err != nil {
			return nil, err
		}
		return &keyservice.EncryptResponse{
			Ciphertext: ciphertext,
		}, nil
	case nil:
		return nil, fmt.Errorf("must provide a key")
	}
	// Fallback to default server for any other request.
	return ks.defaultServer.Encrypt(ctx, req)
}

// Decrypt takes a decrypt request and decrypts the provided ciphertext with
// the provided key, returning the decrypted result.
func (ks Server) Decrypt(ctx context.Context, req *keyservice.DecryptRequest) (*keyservice.DecryptResponse, error) {
	key := req.Key
	switch k := key.KeyType.(type) {
	case *keyservice.Key_PgpKey:
		plaintext, err := ks.decryptWithPgp(k.PgpKey, req.Ciphertext)
		if err != nil {
			return nil, err
		}
		return &keyservice.DecryptResponse{
			Plaintext: plaintext,
		}, nil
	case *keyservice.Key_AgeKey:
		plaintext, err := ks.decryptWithAge(k.AgeKey, req.Ciphertext)
		if err != nil {
			return nil, err
		}
		return &keyservice.DecryptResponse{
			Plaintext: plaintext,
		}, nil
	case *keyservice.Key_VaultKey:
		if ks.vaultToken != "" {
			plaintext, err := ks.decryptWithHCVault(k.VaultKey, req.Ciphertext)
			if err != nil {
				return nil, err
			}
			return &keyservice.DecryptResponse{
				Plaintext: plaintext,
			}, nil
		}
	case *keyservice.Key_KmsKey:
		plaintext, err := ks.decryptWithAWSKMS(k.KmsKey, req.Ciphertext)
		if err != nil {
			return nil, err
		}
		return &keyservice.DecryptResponse{
			Plaintext: plaintext,
		}, nil
	case *keyservice.Key_AzureKeyvaultKey:
		plaintext, err := ks.decryptWithAzureKeyVault(k.AzureKeyvaultKey, req.Ciphertext)
		if err != nil {
			return nil, err
		}
		return &keyservice.DecryptResponse{
			Plaintext: plaintext,
		}, nil
	case *keyservice.Key_GcpKmsKey:
		plaintext, err := ks.decryptWithGCPKMS(k.GcpKmsKey, req.Ciphertext)
		if err != nil {
			return nil, err
		}
		return &keyservice.DecryptResponse{
			Plaintext: plaintext,
		}, nil
	case nil:
		return nil, fmt.Errorf("must provide a key")
	}
	// Fallback to default server for any other request.
	return ks.defaultServer.Decrypt(ctx, req)
}

func (ks *Server) encryptWithPgp(key *keyservice.PgpKey, plaintext []byte) ([]byte, error) {
	pgpKey := pgp.NewMasterKeyFromFingerprint(key.Fingerprint)
	pgp.DisableOpenPGP{}.ApplyToMasterKey(pgpKey)
	if ks.gnuPGHome != "" {
		ks.gnuPGHome.ApplyToMasterKey(pgpKey)
	}
	err := pgpKey.Encrypt(plaintext)
	if err != nil {
		return nil, err
	}
	return []byte(pgpKey.EncryptedKey), nil
}

func (ks *Server) decryptWithPgp(key *keyservice.PgpKey, ciphertext []byte) ([]byte, error) {
	pgpKey := pgp.NewMasterKeyFromFingerprint(key.Fingerprint)
	pgp.DisableOpenPGP{}.ApplyToMasterKey(pgpKey)
	if ks.gnuPGHome != "" {
		ks.gnuPGHome.ApplyToMasterKey(pgpKey)
	}
	pgpKey.EncryptedKey = string(ciphertext)
	plaintext, err := pgpKey.Decrypt()
	return plaintext, err
}

func (ks Server) encryptWithAge(key *keyservice.AgeKey, plaintext []byte) ([]byte, error) {
	// Unlike the other encrypt and decrypt methods, validation of configuration
	// is not required here. As the encryption happens purely based on the
	// Recipient from the key.

	ageKey := age.MasterKey{
		Recipient: key.Recipient,
	}
	if err := ageKey.Encrypt(plaintext); err != nil {
		return nil, err
	}
	return []byte(ageKey.EncryptedKey), nil
}

func (ks *Server) decryptWithAge(key *keyservice.AgeKey, ciphertext []byte) ([]byte, error) {
	ageKey := age.MasterKey{
		Recipient: key.Recipient,
	}
	ks.ageIdentities.ApplyToMasterKey(&ageKey)
	ageKey.EncryptedKey = string(ciphertext)
	plaintext, err := ageKey.Decrypt()
	return plaintext, err
}

func (ks *Server) encryptWithHCVault(key *keyservice.VaultKey, plaintext []byte) ([]byte, error) {
	vaultKey := hcvault.MasterKey{
		VaultAddress: key.VaultAddress,
		EnginePath:   key.EnginePath,
		KeyName:      key.KeyName,
	}
	ks.vaultToken.ApplyToMasterKey(&vaultKey)
	if err := vaultKey.Encrypt(plaintext); err != nil {
		return nil, err
	}
	return []byte(vaultKey.EncryptedKey), nil
}

func (ks *Server) decryptWithHCVault(key *keyservice.VaultKey, ciphertext []byte) ([]byte, error) {
	vaultKey := hcvault.MasterKey{
		VaultAddress: key.VaultAddress,
		EnginePath:   key.EnginePath,
		KeyName:      key.KeyName,
	}
	vaultKey.EncryptedKey = string(ciphertext)
	ks.vaultToken.ApplyToMasterKey(&vaultKey)
	plaintext, err := vaultKey.Decrypt()
	return plaintext, err
}

func (ks *Server) encryptWithAWSKMS(key *keyservice.KmsKey, plaintext []byte) ([]byte, error) {
	awsKey := kmsKeyToMasterKey(key)
	if ks.awsCredsProvider != nil {
		ks.awsCredsProvider.ApplyToMasterKey(&awsKey)
	}
	if err := awsKey.Encrypt(plaintext); err != nil {
		return nil, err
	}
	return []byte(awsKey.EncryptedKey), nil
}

func (ks *Server) decryptWithAWSKMS(key *keyservice.KmsKey, cipherText []byte) ([]byte, error) {
	awsKey := kmsKeyToMasterKey(key)
	awsKey.EncryptedKey = string(cipherText)
	if ks.awsCredsProvider != nil {
		ks.awsCredsProvider.ApplyToMasterKey(&awsKey)
	}
	return awsKey.Decrypt()
}

func (ks *Server) encryptWithAzureKeyVault(key *keyservice.AzureKeyVaultKey, plaintext []byte) ([]byte, error) {
	azureKey := azkv.MasterKey{
		VaultURL: key.VaultUrl,
		Name:     key.Name,
		Version:  key.Version,
	}
	if ks.azureToken == nil {
		// Ensure we use the default token credential if none is provided
		// _without_ shelling out to `az`.
		defaultToken, err := intazkv.DefaultTokenCredential()
		if err != nil {
			return nil, fmt.Errorf("failed to get Azure token credential to encrypt data: %w", err)
		}
		azkv.NewTokenCredential(defaultToken).ApplyToMasterKey(&azureKey)
	} else {
		ks.azureToken.ApplyToMasterKey(&azureKey)
	}
	if err := azureKey.Encrypt(plaintext); err != nil {
		return nil, err
	}
	return []byte(azureKey.EncryptedKey), nil
}

func (ks *Server) decryptWithAzureKeyVault(key *keyservice.AzureKeyVaultKey, ciphertext []byte) ([]byte, error) {
	azureKey := azkv.MasterKey{
		VaultURL: key.VaultUrl,
		Name:     key.Name,
		Version:  key.Version,
	}
	if ks.azureToken == nil {
		// Ensure we use the default token credential if none is provided
		// _without_ shelling out to `az`.
		defaultToken, err := intazkv.DefaultTokenCredential()
		if err != nil {
			return nil, fmt.Errorf("failed to get Azure token credential to decrypt data: %w", err)
		}
		azkv.NewTokenCredential(defaultToken).ApplyToMasterKey(&azureKey)
	} else {
		ks.azureToken.ApplyToMasterKey(&azureKey)
	}
	azureKey.EncryptedKey = string(ciphertext)
	plaintext, err := azureKey.Decrypt()
	return plaintext, err
}

func (ks *Server) encryptWithGCPKMS(key *keyservice.GcpKmsKey, plaintext []byte) ([]byte, error) {
	gcpKey := gcpkms.MasterKey{
		ResourceID: key.ResourceId,
	}
	ks.gcpCredsJSON.ApplyToMasterKey(&gcpKey)
	if err := gcpKey.Encrypt(plaintext); err != nil {
		return nil, err
	}
	return gcpKey.EncryptedDataKey(), nil
}

func (ks *Server) decryptWithGCPKMS(key *keyservice.GcpKmsKey, ciphertext []byte) ([]byte, error) {
	gcpKey := gcpkms.MasterKey{
		ResourceID: key.ResourceId,
	}
	ks.gcpCredsJSON.ApplyToMasterKey(&gcpKey)
	gcpKey.EncryptedKey = string(ciphertext)
	plaintext, err := gcpKey.Decrypt()
	return plaintext, err
}

func kmsKeyToMasterKey(key *keyservice.KmsKey) awskms.MasterKey {
	ctx := make(map[string]*string)
	for k, v := range key.Context {
		value := v
		ctx[k] = &value
	}
	return awskms.MasterKey{
		Arn:               key.Arn,
		Role:              key.Role,
		EncryptionContext: ctx,
	}
}
