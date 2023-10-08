/*
MIT License

Copyright (c) Microsoft Corporation.

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE
*/

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
This package exchanges an ARM access token for an ACR access token on Azure
It has been derived from
https://github.com/Azure/msi-acrpull/blob/main/pkg/authorizer/token_exchanger.go
since the project isn't actively maintained.
*/

package azure

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
)

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	Resource     string `json:"resource"`
	TokenType    string `json:"token_type"`
}

type acrError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type exchanger struct {
	endpoint string
}

// newExchanger returns an Azure Exchanger for Azure Container Registry with
// a given endpoint, for example https://azurecr.io.
func newExchanger(endpoint string) *exchanger {
	return &exchanger{
		endpoint: endpoint,
	}
}

// ExchangeACRAccessToken exchanges an access token for a refresh token with the
// exchange service.
func (e *exchanger) ExchangeACRAccessToken(armToken string) (string, error) {
	// Construct the exchange URL.
	exchangeURL, err := url.Parse(e.endpoint)
	if err != nil {
		return "", err
	}
	exchangeURL.Path = path.Join(exchangeURL.Path, "oauth2/exchange")

	parameters := url.Values{}
	parameters.Add("grant_type", "access_token")
	parameters.Add("service", exchangeURL.Hostname())
	parameters.Add("access_token", armToken)

	resp, err := http.PostForm(exchangeURL.String(), parameters)
	if err != nil {
		return "", fmt.Errorf("failed to send token exchange request: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read the body of the response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// Parse the error response.
		var errors []acrError
		if err = json.Unmarshal(b, &errors); err == nil {
			return "", fmt.Errorf("unexpected status code %d from exchange request: %s",
				resp.StatusCode, errors)
		}

		// Error response could not be parsed, return a generic error.
		return "", fmt.Errorf("unexpected status code %d from exchange request, response body: %s",
			resp.StatusCode, string(b))
	}

	var tokenResp tokenResponse
	if err = json.Unmarshal(b, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode the response: %w, response body: %s", err, string(b))
	}
	return tokenResp.RefreshToken, nil
}
