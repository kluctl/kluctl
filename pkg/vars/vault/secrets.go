package vault

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/vault/api"
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

func GetSecret(vaultAddr string, tokenEnvVar string, secretsPath string) (string, error) {
	client, err := api.NewClient(&api.Config{Address: vaultAddr, HttpClient: httpClient})
	if err != nil {
		return "", fmt.Errorf("failed to create vault %s client", vaultAddr)
	}
	token, _ := os.LookupEnv(tokenEnvVar)
	client.SetToken(token)
	secret, err := client.Logical().Read(secretsPath)
	if err != nil {
		return "", fmt.Errorf("connection to vault %s failed", vaultAddr)
	}
	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("the specified vault secret was not found")
	}
	data, _ := secret.Data["data"].(map[string]interface{})
	jsonData, _ := json.Marshal(data)
	return string(jsonData), nil
}
