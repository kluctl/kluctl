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
	extage "filippo.io/age"
	"github.com/getsops/sops/v3/age"
	"github.com/getsops/sops/v3/azkv"
	"github.com/getsops/sops/v3/gcpkms"
	"github.com/getsops/sops/v3/hcvault"
	"github.com/getsops/sops/v3/keyservice"
	awskms "github.com/getsops/sops/v3/kms"
	"github.com/getsops/sops/v3/pgp"
)

// ServerOption is some configuration that modifies the Server.
type ServerOption interface {
	// ApplyToServer applies this configuration to the given Server.
	ApplyToServer(s *Server)
}

// WithGnuPGHome configures the GnuPG home directory on the Server.
type WithGnuPGHome string

// ApplyToServer applies this configuration to the given Server.
func (o WithGnuPGHome) ApplyToServer(s *Server) {
	s.gnuPGHome = pgp.GnuPGHome(o)
}

// WithVaultToken configures the Hashicorp Vault token on the Server.
type WithVaultToken string

// ApplyToServer applies this configuration to the given Server.
func (o WithVaultToken) ApplyToServer(s *Server) {
	s.vaultToken = hcvault.Token(o)
}

// WithAgeIdentities configures the parsed age identities on the Server.
type WithAgeIdentities []extage.Identity

// ApplyToServer applies this configuration to the given Server.
func (o WithAgeIdentities) ApplyToServer(s *Server) {
	s.ageIdentities = age.ParsedIdentities(o)
}

// WithAWSKeys configures the AWS credentials on the Server
type WithAWSKeys struct {
	CredsProvider *awskms.CredentialsProvider
}

// ApplyToServer applies this configuration to the given Server.
func (o WithAWSKeys) ApplyToServer(s *Server) {
	s.awsCredsProvider = o.CredsProvider
}

// WithGCPCredsJSON configures the GCP service account credentials JSON on the
// Server.
type WithGCPCredsJSON []byte

// ApplyToServer applies this configuration to the given Server.
func (o WithGCPCredsJSON) ApplyToServer(s *Server) {
	s.gcpCredsJSON = gcpkms.CredentialJSON(o)
}

// WithAzureToken configures the Azure credential token on the Server.
type WithAzureToken struct {
	Token *azkv.TokenCredential
}

// ApplyToServer applies this configuration to the given Server.
func (o WithAzureToken) ApplyToServer(s *Server) {
	s.azureToken = o.Token
}

// WithDefaultServer configures the fallback default server on the Server.
type WithDefaultServer struct {
	Server keyservice.KeyServiceServer
}

// ApplyToServer applies this configuration to the given Server.
func (o WithDefaultServer) ApplyToServer(s *Server) {
	s.defaultServer = o.Server
}
