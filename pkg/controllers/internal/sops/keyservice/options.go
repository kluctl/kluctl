// Copyright (C) 2022 The Flux authors
//
// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at https://mozilla.org/MPL/2.0/.

package keyservice

import (
	extage "filippo.io/age"
	"go.mozilla.org/sops/v3/age"
	"go.mozilla.org/sops/v3/azkv"
	"go.mozilla.org/sops/v3/gcpkms"
	"go.mozilla.org/sops/v3/hcvault"
	"go.mozilla.org/sops/v3/keyservice"
	"go.mozilla.org/sops/v3/kms"
	"go.mozilla.org/sops/v3/pgp"
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
	CredsProvider *kms.CredentialsProvider
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
