package azure

import (
	"context"
	"fmt"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets"
)

func GetAzureKeyVaultSecret(ctx context.Context, vaultUri string, secretName string) (string, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return "", fmt.Errorf("login to azure not working cannot get Azure credential")
	}
	client, err := azsecrets.NewClient(vaultUri, cred, nil)
	// Get a secret. An empty string version gets the latest version of the secret.
	version := ""
	resp, err := client.GetSecret(ctx, secretName, version, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get the secret %s", secretName)
	}
	return *resp.Value, nil
}
