package vars

import (
	"crypto/tls"
	"fmt"
	"github.com/Azure/go-ntlmssp"
	"github.com/docker/distribution/registry/client/auth/challenge"
	"github.com/kluctl/kluctl/v2/pkg/status"
	"github.com/kluctl/kluctl/v2/pkg/types"
	"github.com/kluctl/kluctl/v2/pkg/utils/uo"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

func (v *VarsLoader) doHttp(httpSource *types.VarsSourceHttp, username string, password string) (*http.Response, string, error) {
	client := &http.Client{
		Transport: ntlmssp.Negotiator{
			RoundTripper: &http.Transport{
				// This disables HTTP2.0 support, as it does not play well together with NTLM
				TLSNextProto: make(map[string]func(string, *tls.Conn) http.RoundTripper),
			},
		},
	}

	method := "GET"
	if httpSource.Method != nil {
		method = *httpSource.Method
	}

	var reqBody io.Reader
	if httpSource.Body != nil {
		reqBody = strings.NewReader(*httpSource.Body)
	}

	req, err := http.NewRequest(method, httpSource.Url.String(), reqBody)
	if err != nil {
		return nil, "", err
	}

	if username != "" || password != "" {
		req.SetBasicAuth(username, password)
	}

	for k, v := range httpSource.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp, string(respBody), fmt.Errorf("http request to %s failed with status code %d", httpSource.Url.String(), resp.StatusCode)
	}

	return resp, string(respBody), nil
}

func (v *VarsLoader) loadHttp(varsCtx *VarsCtx, source *types.VarsSource, rootKey string) error {
	resp, respBody, err := v.doHttp(source.Http, "", "")
	if err != nil && resp != nil && resp.StatusCode == http.StatusUnauthorized {
		chgs := challenge.ResponseChallenges(resp)
		if len(chgs) == 0 {
			return err
		}

		var realms []string
		for _, chg := range chgs {
			if x, ok := chg.Parameters["realm"]; ok {
				if x != "" {
					realms = append(realms, x)
				}
			}
		}

		credsKey := fmt.Sprintf("%s|%s", source.Http.Url.Host, strings.Join(realms, "+"))
		creds, ok := v.credentialsCache[credsKey]
		if !ok {
			username, password, err := status.AskForCredentials(v.ctx, fmt.Sprintf("Please enter credentials for host '%s'", source.Http.Url.Host))
			if err != nil {
				return err
			}
			creds = usernamePassword{
				username: username,
				password: password,
			}
			v.credentialsCache[credsKey] = creds
		}

		resp, respBody, err = v.doHttp(source.Http, creds.username, creds.password)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	var respObj interface{}
	var newVars *uo.UnstructuredObject

	err = yaml.ReadYamlString(respBody, &respObj)
	if err != nil {
		return err
	}
	if err != nil {
		return err
	}
	if source.Http.JsonPath != nil {
		p, err := uo.NewMyJsonPath(*source.Http.JsonPath)
		if err != nil {
			return err
		}
		x, ok := p.GetFirstFromAny(respObj)
		if !ok {
			return fmt.Errorf("%s not found in result from http request %s", *source.Http.JsonPath, source.Http.Url.String())
		}
		s, ok := x.(string)
		if !ok {
			return fmt.Errorf("%s in result of http request %s is not a string", *source.Http.JsonPath, source.Http.Url.String())
		}
		newVars, err = uo.FromString(s)
		if err != nil {
			return err
		}
	} else {
		x, ok := respObj.(map[string]interface{})
		if !ok {
			return fmt.Errorf("result of http request %s is not an object", source.Http.Url.String())
		}
		newVars = uo.FromMap(x)
	}

	if rootKey != "" {
		newVars, _, err = newVars.GetNestedObject(rootKey)
		if err != nil {
			return err
		}
	}

	if newVars != nil {
		v.mergeVars(varsCtx, newVars, rootKey)
	}
	return nil
}
