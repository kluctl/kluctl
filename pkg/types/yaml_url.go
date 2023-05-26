package types

import (
	"encoding/json"
	"github.com/kluctl/kluctl/v2/pkg/yaml"
	"net/url"
)

type YamlUrl struct {
	url.URL
}

func (u *YamlUrl) UnmarshalJSON(b []byte) error {
	var s string
	err := yaml.ReadYamlBytes(b, &s)
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

func (in *YamlUrl) DeepCopyInto(out *YamlUrl) {
	out.URL = in.URL
	if out.URL.User != nil {
		out.URL.User = &*in.URL.User
	}
}
