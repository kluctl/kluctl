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

package azkv

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// DefaultTokenCredential is a modification of azidentity.NewDefaultAzureCredential,
// specifically adapted to not shell out to the Azure CLI.
//
// It attempts to return an azcore.TokenCredential based on the following order:
//
//   - azidentity.NewEnvironmentCredential if environment variables AZURE_CLIENT_ID,
//     AZURE_CLIENT_ID is set with either one of the following: (AZURE_CLIENT_SECRET)
//     or (AZURE_CLIENT_CERTIFICATE_PATH and AZURE_CLIENT_CERTIFICATE_PATH) or
//     (AZURE_USERNAME, AZURE_PASSWORD)
//   - azidentity.WorkloadIdentityCredential if environment variable configuration
//     (AZURE_AUTHORITY_HOST, AZURE_CLIENT_ID, AZURE_FEDERATED_TOKEN_FILE, AZURE_TENANT_ID)
//     is set by the Azure workload identity webhook.
//   - azidentity.ManagedIdentityCredential if only AZURE_CLIENT_ID env variable is set.
func DefaultTokenCredential() (azcore.TokenCredential, error) {
	var (
		azureClientID           = "AZURE_CLIENT_ID"
		azureFederatedTokenFile = "AZURE_FEDERATED_TOKEN_FILE"
		azureAuthorityHost      = "AZURE_AUTHORITY_HOST"
		azureTenantID           = "AZURE_TENANT_ID"
	)

	var errorMessages []string
	options := &azidentity.DefaultAzureCredentialOptions{}

	envCred, err := azidentity.NewEnvironmentCredential(&azidentity.EnvironmentCredentialOptions{
		ClientOptions: options.ClientOptions, DisableInstanceDiscovery: options.DisableInstanceDiscovery},
	)
	if err == nil {
		return envCred, nil
	} else {
		errorMessages = append(errorMessages, "EnvironmentCredential: "+err.Error())
	}

	// workload identity requires values for AZURE_AUTHORITY_HOST, AZURE_CLIENT_ID, AZURE_FEDERATED_TOKEN_FILE, AZURE_TENANT_ID
	haveWorkloadConfig := false
	clientID, haveClientID := os.LookupEnv(azureClientID)
	if haveClientID {
		if file, ok := os.LookupEnv(azureFederatedTokenFile); ok {
			if _, ok := os.LookupEnv(azureAuthorityHost); ok {
				if tenantID, ok := os.LookupEnv(azureTenantID); ok {
					haveWorkloadConfig = true
					workloadCred, err := azidentity.NewWorkloadIdentityCredential(&azidentity.WorkloadIdentityCredentialOptions{
						ClientID:                 clientID,
						TenantID:                 tenantID,
						TokenFilePath:            file,
						ClientOptions:            options.ClientOptions,
						DisableInstanceDiscovery: options.DisableInstanceDiscovery,
					})
					if err == nil {
						return workloadCred, nil
					} else {
						errorMessages = append(errorMessages, "Workload Identity"+": "+err.Error())
					}
				}
			}
		}
	}
	if !haveWorkloadConfig {
		err := errors.New("missing environment variables for workload identity. Check webhook and pod configuration")
		errorMessages = append(errorMessages, fmt.Sprintf("Workload Identity: %s", err))
	}

	o := &azidentity.ManagedIdentityCredentialOptions{ClientOptions: options.ClientOptions}
	if haveClientID {
		o.ID = azidentity.ClientID(clientID)
	}
	miCred, err := azidentity.NewManagedIdentityCredential(o)
	if err == nil {
		return miCred, nil
	} else {
		errorMessages = append(errorMessages, "ManagedIdentity"+": "+err.Error())
	}

	return nil, errors.New(strings.Join(errorMessages, "\n"))
}
