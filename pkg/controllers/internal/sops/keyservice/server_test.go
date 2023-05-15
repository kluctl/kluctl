// Copyright (C) 2022 The Flux authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package keyservice

import (
	"fmt"
	"go.mozilla.org/sops/v3/age"
	"go.mozilla.org/sops/v3/azkv"
	"go.mozilla.org/sops/v3/gcpkms"
	"go.mozilla.org/sops/v3/hcvault"
	"go.mozilla.org/sops/v3/kms"
	"go.mozilla.org/sops/v3/pgp"
	"os"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/aws/aws-sdk-go-v2/credentials"
	. "github.com/onsi/gomega"
	"go.mozilla.org/sops/v3/keyservice"
	"golang.org/x/net/context"
)

func TestServer_EncryptDecrypt_PGP(t *testing.T) {
	const (
		mockPublicKey   = "testdata/public.gpg"
		mockPrivateKey  = "testdata/private.gpg"
		mockFingerprint = "B59DAF469E8C948138901A649732075EA221A7EA"
	)

	g := NewWithT(t)

	gnuPGHome, err := pgp.NewGnuPGHome()
	g.Expect(err).ToNot(HaveOccurred())
	t.Cleanup(func() {
		_ = os.RemoveAll(gnuPGHome.String())
	})
	g.Expect(gnuPGHome.ImportFile(mockPublicKey)).To(Succeed())

	s := NewServer(WithGnuPGHome(gnuPGHome))
	key := KeyFromMasterKey(pgp.NewMasterKeyFromFingerprint(mockFingerprint))
	dataKey := []byte("some data key")
	encResp, err := s.Encrypt(context.TODO(), &keyservice.EncryptRequest{
		Key:       &key,
		Plaintext: dataKey,
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(encResp.Ciphertext).ToNot(BeEmpty())
	g.Expect(encResp.Ciphertext).ToNot(Equal(dataKey))

	g.Expect(gnuPGHome.ImportFile(mockPrivateKey)).To(Succeed())
	decResp, err := s.Decrypt(context.TODO(), &keyservice.DecryptRequest{
		Key:        &key,
		Ciphertext: encResp.Ciphertext,
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(decResp.Plaintext).To(Equal(dataKey))
}

func TestServer_EncryptDecrypt_age(t *testing.T) {
	g := NewWithT(t)

	const (
		mockRecipient string = "age1lzd99uklcjnc0e7d860axevet2cz99ce9pq6tzuzd05l5nr28ams36nvun"
		mockIdentity  string = "AGE-SECRET-KEY-1G0Q5K9TV4REQ3ZSQRMTMG8NSWQGYT0T7TZ33RAZEE0GZYVZN0APSU24RK7"
	)

	s := NewServer()
	key := KeyFromMasterKey(&age.MasterKey{Recipient: mockRecipient})
	dataKey := []byte("some data key")
	encResp, err := s.Encrypt(context.TODO(), &keyservice.EncryptRequest{
		Key:       &key,
		Plaintext: dataKey,
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(encResp.Ciphertext).ToNot(BeEmpty())
	g.Expect(encResp.Ciphertext).ToNot(Equal(dataKey))

	i := make(age.ParsedIdentities, 0)
	g.Expect(i.Import(mockIdentity)).To(Succeed())

	s = NewServer(WithAgeIdentities(i))
	decResp, err := s.Decrypt(context.TODO(), &keyservice.DecryptRequest{
		Key:        &key,
		Ciphertext: encResp.Ciphertext,
	})
	g.Expect(err).ToNot(HaveOccurred())
	g.Expect(decResp.Plaintext).To(Equal(dataKey))
}

func TestServer_EncryptDecrypt_HCVault(t *testing.T) {
	g := NewWithT(t)

	s := NewServer(WithVaultToken("token"))
	key := KeyFromMasterKey(hcvault.NewMasterKey("https://example.com", "engine-path", "key-name"))
	_, err := s.Encrypt(context.TODO(), &keyservice.EncryptRequest{
		Key: &key,
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to encrypt sops data key to Vault transit backend"))

	_, err = s.Decrypt(context.TODO(), &keyservice.DecryptRequest{
		Key: &key,
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to decrypt sops data key from Vault transit backend"))
}

func TestServer_EncryptDecrypt_HCVault_Fallback(t *testing.T) {
	g := NewWithT(t)

	fallback := NewMockKeyServer()
	s := NewServer(WithDefaultServer{Server: fallback})

	key := KeyFromMasterKey(hcvault.NewMasterKey("https://example.com", "engine-path", "key-name"))
	encReq := &keyservice.EncryptRequest{
		Key:       &key,
		Plaintext: []byte("some data key"),
	}
	_, err := s.Encrypt(context.TODO(), encReq)
	g.Expect(err).To(HaveOccurred())
	g.Expect(fallback.encryptReqs).To(HaveLen(1))
	g.Expect(fallback.encryptReqs).To(ContainElement(encReq))
	g.Expect(fallback.decryptReqs).To(HaveLen(0))

	fallback = NewMockKeyServer()
	s = NewServer(WithDefaultServer{Server: fallback})
	decReq := &keyservice.DecryptRequest{
		Key:        &key,
		Ciphertext: []byte("some ciphertext"),
	}
	_, err = s.Decrypt(context.TODO(), decReq)
	g.Expect(fallback.decryptReqs).To(HaveLen(1))
	g.Expect(fallback.decryptReqs).To(ContainElement(decReq))
	g.Expect(fallback.encryptReqs).To(HaveLen(0))
}

func TestServer_EncryptDecrypt_awskms(t *testing.T) {
	g := NewWithT(t)
	s := NewServer(WithAWSKeys{
		CredsProvider: kms.NewCredentialsProvider(credentials.StaticCredentialsProvider{}),
	})

	key := KeyFromMasterKey(kms.NewMasterKeyFromArn("arn:aws:kms:us-west-2:107501996527:key/612d5f0p-p1l3-45e6-aca6-a5b005693a48", nil, ""))
	_, err := s.Encrypt(context.TODO(), &keyservice.EncryptRequest{
		Key: &key,
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to encrypt sops data key with AWS KMS"))

	_, err = s.Decrypt(context.TODO(), &keyservice.DecryptRequest{
		Key: &key,
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to decrypt sops data key with AWS KMS"))
}

func TestServer_EncryptDecrypt_azkv(t *testing.T) {
	g := NewWithT(t)

	identity, err := azidentity.NewDefaultAzureCredential(nil)
	g.Expect(err).ToNot(HaveOccurred())
	s := NewServer(WithAzureToken{Token: azkv.NewTokenCredential(identity)})

	key := KeyFromMasterKey(azkv.NewMasterKey("", "", ""))
	_, err = s.Encrypt(context.TODO(), &keyservice.EncryptRequest{
		Key: &key,
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to encrypt sops data key with Azure Key Vault"))

	_, err = s.Decrypt(context.TODO(), &keyservice.DecryptRequest{
		Key: &key,
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to decrypt sops data key with Azure Key Vault"))

}

func TestServer_EncryptDecrypt_azkv_Fallback(t *testing.T) {
	g := NewWithT(t)

	fallback := NewMockKeyServer()
	s := NewServer(WithDefaultServer{Server: fallback})

	key := KeyFromMasterKey(azkv.NewMasterKey("", "", ""))
	encReq := &keyservice.EncryptRequest{
		Key:       &key,
		Plaintext: []byte("some data key"),
	}
	_, err := s.Encrypt(context.TODO(), encReq)
	g.Expect(err).To(HaveOccurred())
	g.Expect(fallback.encryptReqs).To(HaveLen(1))
	g.Expect(fallback.encryptReqs).To(ContainElement(encReq))
	g.Expect(fallback.decryptReqs).To(HaveLen(0))

	fallback = NewMockKeyServer()
	s = NewServer(WithDefaultServer{Server: fallback})

	decReq := &keyservice.DecryptRequest{
		Key:        &key,
		Ciphertext: []byte("some ciphertext"),
	}
	_, err = s.Decrypt(context.TODO(), decReq)
	g.Expect(fallback.decryptReqs).To(HaveLen(1))
	g.Expect(fallback.decryptReqs).To(ContainElement(decReq))
	g.Expect(fallback.encryptReqs).To(HaveLen(0))
}

func TestServer_EncryptDecrypt_gcpkms(t *testing.T) {
	g := NewWithT(t)

	creds := `{ "client_id": "<client-id>.apps.googleusercontent.com",
 		"client_secret": "<secret>",
		"type": "authorized_user"}`
	s := NewServer(WithGCPCredsJSON([]byte(creds)))

	resourceID := "projects/test-flux/locations/global/keyRings/test-flux/cryptoKeys/sops"
	key := KeyFromMasterKey(gcpkms.NewMasterKeyFromResourceID(resourceID))
	_, err := s.Encrypt(context.TODO(), &keyservice.EncryptRequest{
		Key: &key,
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to encrypt sops data key with GCP KMS"))

	_, err = s.Decrypt(context.TODO(), &keyservice.DecryptRequest{
		Key: &key,
	})
	g.Expect(err).To(HaveOccurred())
	g.Expect(err.Error()).To(ContainSubstring("failed to decrypt sops data key with GCP KMS"))

}

func TestServer_EncryptDecrypt_Nil_KeyType(t *testing.T) {
	g := NewWithT(t)

	s := NewServer(WithDefaultServer{NewMockKeyServer()})

	expectErr := fmt.Errorf("must provide a key")

	_, err := s.Encrypt(context.TODO(), &keyservice.EncryptRequest{Key: &keyservice.Key{KeyType: nil}})
	g.Expect(err).To(Equal(expectErr))

	_, err = s.Decrypt(context.TODO(), &keyservice.DecryptRequest{Key: &keyservice.Key{KeyType: nil}})
	g.Expect(err).To(Equal(expectErr))
}
