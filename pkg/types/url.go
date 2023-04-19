package types

import (
	"encoding/json"
	"net/url"
)

type YamlUrl struct {
	url.URL
}

func (u *YamlUrl) UnmarshalJSON(b []byte) error {
	var s string
	err := json.Unmarshal(b, &s)
	if err != nil {
		return err
	}
	u2, err := url.Parse(s)
	if err != nil {
		return err
	}
	u.URL = *u2
	return err
}

func (u YamlUrl) MarshalJSON() ([]byte, error) {
	return json.Marshal(u.String())
}
