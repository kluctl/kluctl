package vault

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hashicorp/vault/api"
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

func GetSecret(server string, path string) (*string, error) {
	client, err := api.NewClient(&api.Config{Address: server, HttpClient: httpClient})
	if err != nil {
		return nil, fmt.Errorf("failed to create vault %s client", server)
	}
	secret, err := client.Logical().Read(path)
	if err != nil {
		return nil, fmt.Errorf("reading from vault failed: %v", err)
	}
	if secret == nil || secret.Data == nil {
		return nil, nil
	}
	data, _ := secret.Data["data"].(map[string]interface{})
	jsonData, _ := json.Marshal(data)
	ret := string(jsonData)
	return &ret, nil
}
