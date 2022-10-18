package types

import (
	"net/url"
)

type YamlUrl struct {
	url.URL
}

func (u *YamlUrl) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	err := unmarshal(&s)
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

func (u YamlUrl) MarshalYAML() (interface{}, error) {
	return u.String(), nil
}
