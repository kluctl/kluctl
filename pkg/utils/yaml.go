package utils

import (
	"bytes"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"os"
	"strings"
)

func ReadYamlFile(p string, o interface{}) error {
	r, err := os.Open(p)
	if err != nil {
		return fmt.Errorf("opening %v failed: %w", p, err)
	}
	defer r.Close()

	err = ReadYamlStream(r, o)
	if err != nil {
		return fmt.Errorf("unmarshalling %v failed: %w", p, err)
	}
	return nil
}

func ReadYamlString(s string, o interface{}) error {
	return ReadYamlStream(strings.NewReader(s), o)
}

func ReadYamlStream(r io.Reader, o interface{}) error {
	d := yaml.NewDecoder(r)
	d.KnownFields(true)
	err := d.Decode(o)
	return err
}

func ReadYamlAllFile(p string) ([]interface{}, error) {
	r, err := os.Open(p)
	if err != nil {
		return nil, fmt.Errorf("opening %v failed: %w", p, err)
	}
	defer r.Close()

	return ReadYamlAllStream(r)
}

func ReadYamlAllString(s string) ([]interface{}, error) {
	return ReadYamlAllStream(strings.NewReader(s))
}

func ReadYamlAllBytes(b []byte) ([]interface{}, error) {
	return ReadYamlAllStream(bytes.NewReader(b))
}

func ReadYamlAllStream(r io.Reader) ([]interface{}, error) {
	d := yaml.NewDecoder(r)
	d.KnownFields(true)

	var l []interface{}
	for true {
		var o interface{}
		err := d.Decode(&o)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		l = append(l, o)
	}

	return l, nil
}

func WriteYamlString(o interface{}) (string, error) {
	return WriteYamlAllString([]interface{}{o})
}

func WriteYamlFile(p string, o interface{}) error {
	return WriteYamlAllFile(p, []interface{}{o})
}

func WriteYamlAllFile(p string, l []interface{}) error {
	w, err := os.Create(p)
	if err != nil {
		return err
	}
	defer w.Close()

	return WriteYamlAllStream(w, l)
}

func WriteYamlAllBytes(l []interface{}) ([]byte, error) {
	w := bytes.NewBuffer(nil)

	err := WriteYamlAllStream(w, l)
	if err != nil {
		return nil, err
	}
	return w.Bytes(), nil
}

func WriteYamlAllString(l []interface{}) (string, error) {
	b, err := WriteYamlAllBytes(l)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func WriteYamlAllStream(w io.Writer, l []interface{}) error {
	enc := yaml.NewEncoder(w)
	defer enc.Close()
	enc.SetIndent(2)

	for _, o := range l {
		err := enc.Encode(o)
		if err != nil {
			return err
		}
	}
	return nil
}
